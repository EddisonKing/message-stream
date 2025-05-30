package messagestream

import (
	"crypto/rsa"
	"io"
	"net"
	"time"

	otw "github.com/EddisonKing/on-the-wire"
)

// Message Stream supporting Send and Receive operations.
type MessageStream struct {
	tgtPubKey            *rsa.PublicKey
	sender               io.Writer
	receiver             io.Reader
	output               chan *Message
	errors               chan error
	receivedNonceHistory *nonceManager
	sentNonceHistory     *nonceManager
	sendMsgFn            func(Message, io.Writer) error
	readMsgFn            func(io.Reader) (Message, error)
	opts                 *MessageStreamOptions
	stopReading          chan bool
	closed               bool
	conn                 net.Conn
}

func (ms *MessageStream) logDebug(msg string, args ...any) {
	if ms.opts.Logger != nil {
		msgArgs := append(args, "StreamID", ms.opts.ID)
		ms.opts.Logger.Debug(msg, msgArgs...)
	}
}

func (ms *MessageStream) logError(err error, args ...any) error {
	msgArgs := make([]any, 0)
	msgArgs = append(msgArgs, "StreamID", ms.opts.ID)
	if len(args)%2 != 0 {
		msgArgs = append(msgArgs, "Message")
	}
	msgArgs = append(msgArgs, args...)

	if ms.opts.Logger != nil {
		ms.opts.Logger.Error(err.Error(), msgArgs...)
	}

	return err
}

// Create a new Message Stream from anything that implements io.ReadWriter.
func New(rw io.ReadWriter, opts *MessageStreamOptions) (*MessageStream, error) {
	ms, err := NewFrom(rw, rw, opts)
	if err != nil {
		return nil, err
	}

	if conn, ok := rw.(net.Conn); ok {
		ms.logDebug("Message Stream created over net connection. Storing connection reference")
		ms.conn = conn
	}

	return ms, nil
}

// Create a new Message Stream from an individual io.Reader and io.Writer.
func NewFrom(sender io.Writer, receiver io.Reader, opts *MessageStreamOptions) (*MessageStream, error) {
	if opts == nil {
		opts = NewMessageStreamOptions()
	}

	if opts.DeepLogging {
		otw.SetLogger(opts.Logger)
	}

	output := make(chan *Message, 50)
	errs := make(chan error, 30)

	ms := &MessageStream{
		opts:                 opts,
		sender:               sender,
		receiver:             receiver,
		output:               output,
		errors:               errs,
		receivedNonceHistory: newNonceManager(defaultNonceTTL, "Received", opts.Logger),
		sentNonceHistory:     newNonceManager(defaultNonceTTL, "Sent", opts.Logger),
		stopReading:          make(chan bool, 1),
		closed:               false,
	}

	ms.logDebug("Creating a new Message Stream...")
	ms.logDebug("Message Stream options", "Options", opts)

	// Connect message containing this end's public key
	if ms.opts.UseAsymmetricEncryption {
		ms.logDebug("Asymmetric encryption requested")

		if ms.opts.PublicKey == nil {
			return nil, ms.logError(ErrMissingPublicKey)
		}

		if ms.opts.PrivateKey == nil {
			return nil, ms.logError(ErrMissingPrivateKey)
		}

		ms.logDebug("Exchanging public keys...")
		keyExchangeReceive, keyExchangeSend := otw.New[rsa.PublicKey]().
			UseJSONEncoding().
			UseNonce(ms.sentNonceHistory.Generate, ms.receivedNonceHistory.NotContains).
			UseCompression().
			UseTimeout(ms.opts.KeyExchangeTimeout).
			Build()

		ms.logDebug("Sending public key...")
		if err := keyExchangeSend(*ms.opts.PublicKey, ms.sender); err != nil {
			return nil, ms.logError(ErrFailedToSendPublicKey, err.Error())
		}

		// Receive target's public key
		ms.logDebug("Waiting for public key from client...")
		receivedPubKey, err := keyExchangeReceive(ms.receiver)
		if err != nil {
			return nil, ms.logError(ErrFailedToReceivePublicKey, err.Error())
		}

		ms.logDebug("Received public key from client")
		ms.tgtPubKey = &receivedPubKey

	}

	messagePipeline := otw.New[Message]().
		UseJSONEncoding().
		UseNonce(ms.sentNonceHistory.Generate, ms.receivedNonceHistory.NotContains)

	if opts.UseAsymmetricEncryption {
		messagePipeline.UseAsymmetricEncryption(func() *rsa.PublicKey {
			return ms.tgtPubKey
		}, func() *rsa.PrivateKey {
			return ms.opts.PrivateKey
		})
	}

	rmf, smf := messagePipeline.
		UseCompression().
		UseTimeout(time.Second * 15).
		Build()

	ms.readMsgFn = rmf
	ms.sendMsgFn = smf

	go func() {
		ms.logDebug("Waiting for Messages from client...")
		for {
			result := make(chan *Message, 1)
			defer close(result)

			go func() {
				msg, err := ms.receiveMessage()
				if err != nil {
					ms.errors <- err
					return
				}

				ms.logDebug("Received Message", "Type", msg.Type)
				result <- msg
			}()

			select {
			case <-ms.stopReading:
				return
			case msg := <-result:
				ms.output <- msg
			}
		}
	}()

	ms.logDebug("Message Stream successfully negotiated")
	ms.closed = false
	return ms, nil
}

// Returns the RSA Public Key that was negotiate from the other end of the Message Stream if encryption was used. The key is nil if no Public Key was sent.
func (ms *MessageStream) GetRecipientPublicKey() *rsa.PublicKey {
	return ms.tgtPubKey
}

// Returns the underlying `net.Conn` if the Message Stream was created from one. nil otherwise.
func (ms *MessageStream) GetConnection() net.Conn {
	return ms.conn
}

// Terminates any internal channels preventing sending and receiving on this Message Stream.
func (ms *MessageStream) Close() {
	if ms.closed {
		return
	}

	ms.logDebug("Closing Message Stream")
	ms.closed = true
	ms.stopReading <- true
	close(ms.output)
	close(ms.stopReading)
}

// Sends a Message on the io.Writer portion of the Message Stream.
//
// Returns an error if it fails serialise the metadata or payload, write data to the underlying `io.Writer` or generate a nonce.
//
// If proxying is enabled, the Message will be proxied through the designated addresses if they support Message Streams
func (ms *MessageStream) SendMessage(t MessageType, metadata map[string]any, payload any, proxies ...string) error {
	useProxy := len(proxies) > 0

	if useProxy && !ms.opts.AllowProxying {
		return ms.logError(ErrProxyingNotAllowed, "Message proxying is disabled for security reasons by default. If you want to be able to proxy Messages, pass in options that have AllowProxying set to true", "CurrentOptions", ms.opts)
	}

	for _, proxy := range proxies {
		if _, err := net.ResolveTCPAddr("tcp", proxy); err != nil {
			return ms.logError(ErrProxyAddressUnresolvable, "Proxy address is not a resolvable TCP address", "ProxyAddr", proxy, "Error", err)
		}
	}

	if !useProxy {
		ms.logDebug("Sending Message", "Type", t, "Metadata", metadata)
		m, err := newMessage(t, metadata, payload)
		if err != nil {
			return err
		}

		return ms.sendMessage(m)
	} else {
		msg, err := newMessage(t, metadata, payload)
		if err != nil {
			return err
		}

		proxiedMsg, err := newMessage(msxProxy, map[string]any{
			msxProxyDstMetaKey: proxies,
		}, msg)
		if err != nil {
			return err
		}

		ms.logDebug("Sending Proxied Message", "Type", t, "Metadata", metadata, "Path", proxies)

		return ms.sendMessage(proxiedMsg)
	}
}

// Forward an existing Message. This is useful in a situation where multiple Message Streams are being used and a received Message needs to be passed to a different Message Stream.
//
// Returns an error if it fails to write data to the underlying `io.Writer` or generate a nonce.
func (ms *MessageStream) ForwardMessage(msg *Message) error {
	ms.logDebug("Forwarding Message", "Type", msg.Type, "Metadata", msg.Metadata)
	return ms.sendMessage(msg)
}

// Returns a channel where incoming Messages can be received.
func (ms *MessageStream) Receiver() <-chan *Message {
	return ms.output
}

// Returns a channel where any errors generated by the Message Stream's operations will be sent.
func (ms *MessageStream) Errors() <-chan error {
	return ms.errors
}

func (ms *MessageStream) sendMessage(m *Message) error {
	if ms.closed {
		return ErrMessageStreamClosed
	}

	if err := ms.sendMsgFn(*m, ms.sender); err != nil {
		return err
	}

	return nil
}

func (ms *MessageStream) receiveMessage() (*Message, error) {
	for {
		ms.logDebug("Beginning Message receive...")
		var msg Message
		var err error
		for {
			msg, err = ms.readMsgFn(ms.receiver)
			if err == nil {
				break
			}
		}

		if msg.Type == msxProxy {
			go ms.handleProxiedMessage(msg)
			continue
		}

		ms.logDebug("Received Message", "Type", msg.Type, "Metadata", msg.Metadata)
		return &msg, nil
	}
}

func (ms *MessageStream) handleProxiedMessage(msg Message) {
	ms.logDebug("Received Proxy Message", "Type", msg.Type)

	payload, meta, err := Unwrap[Message](&msg)
	if err != nil {
		ms.errors <- ms.logError(ErrFailedToUnwrapProxyMessage, err.Error())
		return
	}

	dstListProp, exists := meta[msxProxyDstMetaKey]
	if !exists {
		// This node should be the intended recipient
		ms.output <- &msg
		return
	}

	dstListArr, ok := dstListProp.([]any)
	if !ok {
		ms.logError(ErrMalformedProxies, "Proxy Message contains a malformed proxy list. Expected []string. No choice but to drop", "Proxies", dstListProp)
		return
	}

	dstList := make([]string, len(dstListArr))
	for i, dstProp := range dstListArr {
		dst, ok := dstProp.(string)
		if !ok {
			ms.logError(ErrMalformedProxies, "Proxy Message proxy list contains a malformed entry. Expected string. No choice but to drop", "Proxies", dstProp)
			return
		}

		dstList[i] = dst
	}

	if len(dstList) < 1 {
		// This node should be the intended recipient although, an empty slice seems like an unlikely scenario
		ms.output <- &msg
		return
	}

	nxtHop := dstList[0]
	nxt, err := Dial(nxtHop, ms.opts)
	if err != nil {
		ms.logError(ErrFailedToProxyMessage, err.Error())
		return
	}
	defer nxt.Close()

	if err := nxt.SendMessage(msxProxy, map[string]any{
		msxProxyDstMetaKey: dstList[1:],
	}, payload, dstList[1:]...); err != nil {
		ms.logError(ErrFailedToProxyMessage, err.Error())
		return
	}

	proxyReply := <-nxt.Receiver()
	if err := ms.ForwardMessage(proxyReply); err != nil {
		ms.logError(ErrFailedToProxyReplyMessage, err.Error())
		return
	}
}
