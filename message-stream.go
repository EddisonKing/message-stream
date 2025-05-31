package messagestream

import (
	"crypto/rsa"
	"io"
	"net"
	"sync"
	"time"

	otw "github.com/EddisonKing/on-the-wire"
)

// Message Stream supporting Send and Receive operations.
type MessageStream struct {
	tgtPubKey            *rsa.PublicKey
	sender               io.Writer
	receiver             io.Reader
	output               chan *Message
	receivedNonceHistory *nonceManager
	sentNonceHistory     *nonceManager
	sendMsgFn            func(Message, io.Writer) error
	readMsgFn            func(io.Reader) (Message, error)
	opts                 *MessageStreamOptions
	stopReading          chan bool
	closed               bool
	conn                 net.Conn

	mu               *sync.Mutex
	onetimeCallbacks map[MessageType][]func(*Message)
	callbacks        map[MessageType][]func(*Message)
	onetimeReads     map[MessageType][]chan *Message
	reads            map[MessageType][]chan *Message
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

	ms := &MessageStream{
		opts:                 opts,
		sender:               sender,
		receiver:             receiver,
		output:               output,
		receivedNonceHistory: newNonceManager(defaultNonceTTL, "Received", opts.Logger),
		sentNonceHistory:     newNonceManager(defaultNonceTTL, "Sent", opts.Logger),
		stopReading:          make(chan bool, 1),
		closed:               false,

		mu:               &sync.Mutex{},
		onetimeCallbacks: make(map[MessageType][]func(*Message)),
		onetimeReads:     make(map[MessageType][]chan *Message),
		callbacks:        make(map[MessageType][]func(*Message)),
		reads:            make(map[MessageType][]chan *Message),
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

	go ms.handleIncomingMessages()

	ms.logDebug("Message Stream successfully negotiated")
	ms.closed = false
	return ms, nil
}

// Creates a single-use callback to handle one Message of a specific Message Type. When a Message of this type is received the callback will be executed and pass the received Message. The callback will then be discarded from the call list.
func (ms *MessageStream) OnOne(msgType MessageType, callback func(*Message)) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.onetimeCallbacks[msgType]; !exists {
		ms.onetimeCallbacks[msgType] = make([]func(*Message), 0)
	}

	ms.onetimeCallbacks[msgType] = append(ms.onetimeCallbacks[msgType], callback)
}

// Creates a callback to handle Messages of a specific Message Type. When a Message of this type is received the callback will be executed and pass the received Message. Messages that are to be consumed in this way will NOT end up at the Receiver.
func (ms *MessageStream) On(msgType MessageType, callback func(*Message)) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.callbacks[msgType]; !exists {
		ms.callbacks[msgType] = make([]func(*Message), 0)
	}

	ms.callbacks[msgType] = append(ms.callbacks[msgType], callback)
}

// Reads one specific Message of the given Message Type. This call blocks this goroutine until this Message Type is received. Other messages can still be received and processed in other goroutines.
func (ms *MessageStream) ReadOne(msgType MessageType) *Message {
	ms.mu.Lock()

	if _, exists := ms.onetimeReads[msgType]; !exists {
		ms.onetimeReads[msgType] = make([]chan *Message, 0)
	}

	result := make(chan *Message)

	ms.reads[msgType] = append(ms.reads[msgType], result)

	ms.mu.Unlock()

	return <-result
}

// Returns a channel that will only read the specified Message Type. This will prevent this Message Type being received at the Receiver, however, you can create multiple readers of this Message Type still.
func (ms *MessageStream) Read(msgType MessageType) <-chan *Message {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.reads[msgType]; !exists {
		ms.reads[msgType] = make([]chan *Message, 0)
	}

	result := make(chan *Message)

	ms.reads[msgType] = append(ms.reads[msgType], result)

	return result
}

func (ms *MessageStream) handleIncomingMessages() {
	ms.logDebug("Waiting for Messages from client...")
	for {
		result := make(chan *Message, 1)
		defer close(result)

		go func() {
			msg := ms.receiveMessage()
			ms.logDebug("Received Message", "Type", msg.Type)
			result <- msg
		}()

		select {
		case <-ms.stopReading:
			return
		case msg := <-result:
			absorbed := false

			ms.mu.Lock()
			// Check for one time reads
			if onetimeReads, exists := ms.onetimeReads[msg.Type]; exists {
				for _, onetimeRead := range onetimeReads {
					onetimeRead <- msg
				}

				ms.onetimeReads[msg.Type] = make([]chan *Message, 0)
			}

			// Check for one time callbacks
			if onetimeCallbacks, exists := ms.onetimeCallbacks[msg.Type]; exists {
				for _, onetimeCallback := range onetimeCallbacks {
					go onetimeCallback(msg)
				}

				ms.onetimeCallbacks[msg.Type] = make([]func(*Message), 0)
			}

			// Check for reads
			if reads, exists := ms.reads[msg.Type]; exists {
				for _, read := range reads {
					read <- msg
				}

				absorbed = true
			}

			// Check for callbacks
			if callbacks, exists := ms.callbacks[msg.Type]; exists {
				for _, callback := range callbacks {
					go callback(msg)
				}

				absorbed = true
			}

			ms.mu.Unlock()

			if !absorbed {
				// Only happens at ends up at receiver if not captured by callback or read
				ms.output <- msg
			}
		}
	}
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

		proxiedMsg, err := newMessage(msxProxy, nil, msg, proxies...)
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

func (ms *MessageStream) sendMessage(m *Message) error {
	if ms.closed {
		return ErrMessageStreamClosed
	}

	if err := ms.sendMsgFn(*m, ms.sender); err != nil {
		return err
	}

	return nil
}

func (ms *MessageStream) receiveMessage() *Message {
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
		return &msg
	}
}

func (ms *MessageStream) handleProxiedMessage(msg Message) {
	ms.logDebug("Received Proxy Message", "Type", msg.Type, "Proxies", msg.Proxies, "Message", msg)

	payload, meta, err := Unwrap[Message](&msg)
	if err != nil {
		ms.logError(ErrFailedToUnwrapProxyMessage, err.Error())
		return
	}

	if len(msg.Proxies) < 1 {
		ms.logDebug("Message intended for this Message Stream", "Type", msg.Type)
		// This node should be the intended recipient although, an empty slice seems like an unlikely scenario
		ms.output <- &msg
		return
	}

	ms.logDebug("Message needs to be proxied through", "Path", msg.Proxies[1:], "Next", msg.Proxies[0])
	nxtHop := msg.Proxies[0]
	nxt, err := Dial(nxtHop, ms.opts)
	if err != nil {
		ms.logError(ErrFailedToProxyMessage, err.Error())
		return
	}
	defer nxt.Close()

	if err := nxt.SendMessage(msxProxy, meta, payload, msg.Proxies[1:]...); err != nil {
		ms.logError(ErrFailedToProxyMessage, err.Error())
		return
	}

	proxyReply := <-nxt.Receiver()
	if err := ms.ForwardMessage(proxyReply); err != nil {
		ms.logError(ErrFailedToProxyReplyMessage, err.Error())
		return
	}
}
