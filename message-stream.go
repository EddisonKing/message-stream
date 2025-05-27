package messagestream

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	otw "github.com/EddisonKing/on-the-wire"
)

var (
	ErrMessageStreamClosed = errors.New("Message Stream is closed")
	ErrMissingPublicKey    = errors.New("missing public key")
	ErrMissingPrivateKey   = errors.New("missing private key")
	ErrProxyingNotAllowed  = errors.New("proxying is not allowed")
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
}

// Create a new Message Stream from anything that implements io.ReadWriter.
func New(rw io.ReadWriter, opts *MessageStreamOptions) (*MessageStream, error) {
	return NewFrom(rw, rw, opts)
}

// Create a new Message Stream from an individual io.Reader and io.Writer.
func NewFrom(sender io.Writer, receiver io.Reader, opts *MessageStreamOptions) (*MessageStream, error) {
	if opts == nil {
		opts = NewMessageStreamOptions()
		opts.Logger.Debug("Options was nil. Using defaults", "Options", opts, "StreamID", opts.ID)
	}

	opts.Logger.Debug("Creating a new Message Stream...", "StreamID", opts.ID)
	opts.Logger.Debug("Message Stream options", "Options", opts, "StreamID", opts.ID)

	if opts.DeepLogging {
		// otw.SetLogger(opts.Logger)
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

	// Connect message containing this end's public key
	if ms.opts.UseAsymmetricEncryption {
		ms.opts.Logger.Debug("Asymmetric encryption requested", "StreamID", opts.ID)

		if ms.opts.PublicKey == nil {
			ms.opts.Logger.Error("Public Key was nil and needs to be set for asymmetric encryption", "StreamID", opts.ID)
			return nil, ErrMissingPublicKey
		}

		if ms.opts.PrivateKey == nil {
			ms.opts.Logger.Error("Private Key was nil and needs to be set for asymmetric encryption", "StreamID", opts.ID)
			return nil, ErrMissingPrivateKey
		}

		ms.opts.Logger.Debug("Exchanging public keys...", "StreamID", ms.opts.ID)
		keyExchangeReceive, keyExchangeSend := otw.New[rsa.PublicKey]().
			UseJSONEncoding().
			UseNonce(ms.sentNonceHistory.Generate, ms.receivedNonceHistory.NotContains).
			UseCompression().
			UseTimeout(ms.opts.KeyExchangeTimeout).
			Build()

		ms.opts.Logger.Debug("Sending public key...", "StreamID", ms.opts.ID)
		if err := keyExchangeSend(*ms.opts.PublicKey, ms.sender); err != nil {
			ms.opts.Logger.Error("Failed to send public key during public key exchange", "Error", err)
			return nil, err
		}

		// Receive target's public key
		ms.opts.Logger.Debug("Waiting for public key from client...", "StreamID", ms.opts.ID)
		receivedPubKey, err := keyExchangeReceive(ms.receiver)
		if err != nil {
			ms.opts.Logger.Error("Failed to recieve public key during public key exchange", "Error", err)
			return nil, err
		}

		ms.opts.Logger.Debug("Received public key from client", "StreamID", ms.opts.ID)
		ms.tgtPubKey = &receivedPubKey

		rmf, smf := otw.New[Message]().
			UseJSONEncoding().
			UseNonce(ms.sentNonceHistory.Generate, ms.receivedNonceHistory.NotContains).
			UseAsymmetricEncryption(func() *rsa.PublicKey {
				return ms.tgtPubKey
			}, func() *rsa.PrivateKey {
				return ms.opts.PrivateKey
			}).
			UseCompression().
			UseTimeout(time.Second * 15).
			Build()

		ms.readMsgFn = rmf
		ms.sendMsgFn = smf
	} else {
		rmf, smf := otw.New[Message]().
			UseJSONEncoding().
			UseNonce(ms.sentNonceHistory.Generate, ms.receivedNonceHistory.NotContains).
			UseCompression().
			UseTimeout(ms.opts.MessageExchangeTimeout).
			Build()

		ms.readMsgFn = rmf
		ms.sendMsgFn = smf
	}

	go func() {
		ms.opts.Logger.Debug("Waiting for Messages from client...", "StreamID", ms.opts.ID)
		for {
			result := make(chan *Message, 1)
			defer close(result)

			go func() {
				msg, err := ms.receiveMessage()
				if err != nil {
					ms.errors <- err
					return
				}

				ms.opts.Logger.Debug("Received Message", "Type", msg.Type, "StreamID", ms.opts.ID)
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

	ms.opts.Logger.Debug("Message Stream successfully negotiated", "StreamID", ms.opts.ID)
	ms.closed = false
	return ms, nil
}

// Returns the RSA Public Key that was negotiate from the other end of the Message Stream if encryption was used. The key is nil if no Public Key was sent.
func (ms *MessageStream) GetRecipientPublicKey() *rsa.PublicKey {
	return ms.tgtPubKey
}

// Terminates any internal channels preventing sending and receiving on this Message Stream.
func (ms *MessageStream) Close() {
	if ms.closed {
		return
	}

	ms.opts.Logger.Debug("Closing Message Stream", "StreamID", ms.opts.ID)
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
		ms.opts.Logger.Error("Message proxying is disabled for security reasons by default. If you want to be able to proxy Messages, pass in options that have AllowProxying set to true", "CurrentOptions", ms.opts)
		return ErrProxyingNotAllowed
	}

	for _, proxy := range proxies {
		if _, err := net.ResolveTCPAddr("tcp", proxy); err != nil {
			ms.opts.Logger.Error("Proxy address is not a resolvable TCP address", "ProxyAddr", proxy, "Error", err)
			return err
		}
	}

	if !useProxy {
		ms.opts.Logger.Debug("Sending Message", "Type", t, "Metadata", metadata, "StreamID", ms.opts.ID)
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

		ms.opts.Logger.Debug("Sending Proxied Message", "Type", t, "Metadata", metadata, "Path", proxies, "StreamID", ms.opts.ID)

		return ms.sendMessage(proxiedMsg)
	}
}

// Forward an existing Message. This is useful in a situation where multiple Message Streams are being used and a received Message needs to be passed to a different Message Stream.
//
// Returns an error if it fails to write data to the underlying `io.Writer` or generate a nonce.
func (ms *MessageStream) ForwardMessage(msg *Message) error {
	ms.opts.Logger.Debug("Forwarding Message", "Type", msg.Type, "Metadata", msg.Metadata, "StreamID", ms.opts.ID)
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
		ms.opts.Logger.Error("Message Stream is closed. Unable to forward.", "StreamID", ms.opts.ID)
		return ErrMessageStreamClosed
	}

	if err := ms.sendMsgFn(*m, ms.sender); err != nil {
		return err
	}

	return nil
}

func (ms *MessageStream) receiveMessage() (*Message, error) {
	for {
		ms.opts.Logger.Debug("Beginning Message receive...", "StreamID", ms.opts.ID)
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

		ms.opts.Logger.Debug("Received Message", "Type", msg.Type, "Metadata", msg.Metadata, "StreamID", ms.opts.ID)
		return &msg, nil
	}
}

func (ms *MessageStream) handleProxiedMessage(msg Message) {
	ms.opts.Logger.Debug("Received Proxy Message", "Type", msg.Type, "StreamID", ms.opts.ID)

	payload, meta, err := Unwrap[Message](&msg)
	if err != nil {
		ms.opts.Logger.Error("Failed to unwrap proxy Message", "Error", err)
		ms.errors <- err
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
		ms.opts.Logger.Error("Proxy Message contains a malformed proxy list. Expected []string. No choice but to drop", "Proxies", dstListProp)
		return
	}

	dstList := make([]string, len(dstListArr))
	for i, dstProp := range dstListArr {
		dst, ok := dstProp.(string)
		if !ok {
			ms.opts.Logger.Error("Proxy Message proxy list contains a malformed entry. Expected string. No choice but to drop", "Proxies", dstProp)
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
		errMsg := fmt.Errorf("failed to negotiate connection to proxy Message Stream: %s", err)
		ms.opts.Logger.Error("Failed to proxy Messge when dialing next proxy", "Error", errMsg)
		return
	}
	defer nxt.Close()

	if err := nxt.SendMessage(msxProxy, map[string]any{
		msxProxyDstMetaKey: dstList[1:],
	}, payload, dstList[1:]...); err != nil {
		ms.opts.Logger.Error("Failed to proxy Message during sending to next proxy", "Error", err)
		return
	}

	proxyReply := <-nxt.Receiver()
	if err := ms.ForwardMessage(proxyReply); err != nil {
		ms.opts.Logger.Error("Failed to send proxy reply Message back to originator", "Error", err)
		return
	}
}
