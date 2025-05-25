package messagestream

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"math"
	"math/big"
	"time"

	otw "github.com/EddisonKing/on-the-wire"
	"github.com/google/uuid"
)

// Message Stream supporting Send and Receive operations.
type MessageStream struct {
	id                   uuid.UUID
	tgtPubKey            *rsa.PublicKey
	pubKey               *rsa.PublicKey
	privKey              *rsa.PrivateKey
	sender               io.Writer
	receiver             io.Reader
	output               chan *Message
	errors               chan error
	done                 chan bool
	receivedNonceHistory map[int]time.Time
	sentNonceHistory     map[int]time.Time
	sendMsgFn            func(Message, io.Writer) error
	readMsgFn            func(io.Reader) (Message, error)
	connected            bool
}

// Create a new Message Stream from anything that implements io.ReadWriter.
func New(rw io.ReadWriter) *MessageStream {
	return NewFrom(rw, rw)
}

// Create a new Message Stream from an individual io.Reader and io.Writer.
//
// Use `SetKeys()` to specify that encryption should be used and pass in the keys
func NewFrom(sender io.Writer, receiver io.Reader) *MessageStream {
	otw.SetLogger(logger)

	id := uuid.New()

	logger.Info("Creating a new Message Stream", "StreamID", id)

	output := make(chan *Message, 50)
	errs := make(chan error, 30)

	msgStream := &MessageStream{
		id:                   id,
		sender:               sender,
		receiver:             receiver,
		output:               output,
		errors:               errs,
		receivedNonceHistory: make(map[int]time.Time),
		sentNonceHistory:     make(map[int]time.Time),
		done:                 make(chan bool, 1),
		connected:            false,
	}

	return msgStream
}

// This enables encryption on the Message Stream. Pass in the private and public RSA keys. The public key will be transferred to the client on Connect.
func (ms *MessageStream) SetKeys(privKey *rsa.PrivateKey, pubKey *rsa.PublicKey) {
	logger.Debug("Setting Private and Public Keys", "PubKey", pubKey)
	ms.privKey = privKey
	ms.pubKey = pubKey
}

// Returns the RSA Public Key that was negotiate from the other end of the Message Stream if encryption was used. The key is nil if no Public Key was sent.
func (ms *MessageStream) GetRecipientPublicKey() *rsa.PublicKey {
	return ms.tgtPubKey
}

// Connect negotiates the Message Stream with the client. If you attempt to SendMessage before Connect is called, Connect will be called automatically.
//
// This negotation includes the public key exchange that enables secure Message transfer.
// An error will be returned if it is unable to exchnage public keys.
func (ms *MessageStream) Connect() error {
	if ms.connected {
		return nil
	}

	// Connect message containing this end's public key
	if ms.privKey != nil && ms.pubKey != nil {
		logger.Debug("Exchanging public keys", "StreamID", ms.id)
		keyExchangeReceive, keyExchangeSend := otw.New[rsa.PublicKey]().
			UseJSONEncoding().
			UseNonce(ms.newNonce, ms.checkReceivedNonce).
			UseCompression().
			UseTimeout(time.Second * 15).
			Build()

		logger.Debug("Sending public key", "StreamID", ms.id)
		if err := keyExchangeSend(*ms.pubKey, ms.sender); err != nil {
			logger.Error("Failed to send public key during public key exchange", "Error", err)
			return err
		}

		// Receive target's public key
		logger.Debug("Waiting for public key from client...", "StreamID", ms.id)
		receivedPubKey, err := keyExchangeReceive(ms.receiver)
		if err != nil {
			logger.Error("Failed to recieve public key during public key exchange", "Error", err)
			return err
		}

		logger.Debug("Received public key from client", "StreamID", ms.id)
		ms.tgtPubKey = &receivedPubKey

		rmf, smf := otw.New[Message]().
			UseJSONEncoding().
			UseNonce(ms.newNonce, ms.checkReceivedNonce).
			UseAsymmetricEncryption(func() *rsa.PublicKey {
				return ms.tgtPubKey
			}, func() *rsa.PrivateKey {
				return ms.privKey
			}).
			UseCompression().
			UseTimeout(time.Second * 15).
			Build()

		ms.readMsgFn = rmf
		ms.sendMsgFn = smf
	} else {
		logger.Warn("No keys set, Message will be sent unencrypted")
		rmf, smf := otw.New[Message]().
			UseJSONEncoding().
			UseNonce(ms.newNonce, ms.checkReceivedNonce).
			UseCompression().
			UseTimeout(time.Second * 15).
			Build()

		ms.readMsgFn = rmf
		ms.sendMsgFn = smf
	}

	go func() {
		logger.Debug("Waiting for Messages from client...", "StreamID", ms.id)
		for {
			result := make(chan *Message, 1)
			defer close(result)

			go func() {
				msg, err := ms.receiveMessage()
				if err != nil {
					ms.errors <- err
					return
				}
				logger.Info("Received Message", "Type", msg.Type, "StreamID", ms.id)
				result <- msg
			}()

			select {
			case <-ms.done:
				return
			case msg := <-result:
				ms.output <- msg
			}
		}
	}()

	logger.Info("Message Stream successfully negotiated", "StreamID", ms.id)
	ms.connected = true
	return nil
}

func (ms *MessageStream) generateNonce() int {
	nonce, err := rand.Int(rand.Reader, big.NewInt(int64(math.MaxInt32)))
	if err != nil {
		logger.Error("Failed to generate nonce", "Error", err, "StreamID", ms.id)
		panic(err)
	}

	logger.Debug("Generated new nonce", "Nonce", nonce.Int64(), "StreamID", ms.id)
	return int(nonce.Int64())
}

func (ms *MessageStream) newNonce() int {
	var n int
	for {
		nonce := ms.generateNonce()

		if err := ms.storeNonce(&ms.sentNonceHistory, nonce); err != nil {
			continue
		}

		n = nonce
		break
	}
	return n
}

const nonceTTL time.Duration = time.Minute * 15 // Not sure about the time, this should be relative to the time it might take to randomly generate the same nonce again

func (ms *MessageStream) storeNonce(nonceHistory *map[int]time.Time, nonce int) error {
	if seenTime, exists := (*nonceHistory)[nonce]; exists {
		if time.Since(seenTime) < nonceTTL {
			logger.Warn("Nonce already stored", "Nonce", nonce, "StreamID", ms.id)
			return fmt.Errorf("nonce %d has already been received, this could be a replay", nonce)
		}
	}
	// Refresh or set nonce TTL
	logger.Debug("Storing new sent nonce", "Nonce", nonce, "StreamID", ms.id)
	(*nonceHistory)[nonce] = time.Now().UTC()
	return nil
}

func (ms *MessageStream) checkReceivedNonce(n int) bool {
	return ms.storeNonce(&ms.receivedNonceHistory, n) == nil
}

// Terminates any internal channels preventing sending and receiving on this Message Stream.
func (ms *MessageStream) Close() {
	logger.Info("Closing Message Stream", "StreamID", ms.id)
	ms.done <- true
	close(ms.output)
	ms.connected = false
}

// Sends a Message on the io.Writer portion of the Message Stream.
//
// Returns an error if it fails serialise the metadata or payload, write data to the underlying `io.Writer` or generate a nonce.
func (ms *MessageStream) SendMessage(t MessageType, metadata map[string]any, payload any) error {
	if !ms.connected {
		logger.Debug("SendMessage called before Message Stream was connected; automatically calling Connect")
		if err := ms.Connect(); err != nil {
			logger.Error("Failed to negotiate Mesage Stream during Connect", "Error", err)
			return err
		}
	}

	logger.Info("Preparing to send Message", "StreamID", ms.id)
	m, err := newMessage(t, metadata, payload)
	if err != nil {
		return err
	}

	logger.Debug("Message created", "Type", t, "StreamID", ms.id)

	return ms.sendMessage(m)
}

// Forward an existing Message. This is useful in a situation where multiple Message Streams are being used and a received Message needs to be passed to a different Message Stream.
//
// Returns an error if it fails to write data to the underlying `io.Writer` or generate a nonce.
func (ms *MessageStream) ForwardMessage(msg *Message) error {
	logger.Info("Forwarding Message", "Type", msg.Type, "StreamID", ms.id)

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
	logger.Debug("Sending message", "Type", m.Type, "StreamID", ms.id)
	if err := ms.sendMsgFn(*m, ms.sender); err != nil {
		return err
	}

	return nil
}

func (ms *MessageStream) receiveMessage() (*Message, error) {
	logger.Debug("Beginning Message receive...", "StreamID", ms.id)
	var msg Message
	var err error
	for {
		msg, err = ms.readMsgFn(ms.receiver)
		if err != nil {
			if err == otw.ErrTimedOut {
				logger.Debug("Timeout passed during receive Message. Trying again.")
				continue
			}
			return nil, err
		}

		break
	}

	return &msg, nil
}
