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
	closed               bool
	receivedNonceHistory map[int]time.Time
	sentNonceHistory     map[int]time.Time
	sendMsgFn            func(Message, io.Writer) (int, error)
	readMsgFn            func(io.Reader) (Message, int, error)
}

// Create a new Message Stream from anything that implements io.ReadWriter.
//
// Returns a reference to the Message Stream, a channel to receive Messages on and a channel to read any errors on.
func New(rw io.ReadWriter) (*MessageStream, error) {
	return NewFrom(rw, rw)
}

// Create a new Message Stream from an individual io.Reader and io.Writer.
//
// Returns a reference to the Message Stream, a channel to receive Messages on and a channel to read any errors on.
func NewFrom(sender io.Writer, receiver io.Reader) (*MessageStream, error) {
	otw.SetLogger(logger)

	id := uuid.New()

	logger.Info("Creating a new Message Stream", "StreamID", id)
	privKey, pubKey := generateRSAKeyPair()

	output := make(chan *Message, 50)
	errs := make(chan error, 30)

	msgStream := &MessageStream{
		id:                   id,
		pubKey:               pubKey,
		privKey:              privKey,
		sender:               sender,
		receiver:             receiver,
		output:               output,
		closed:               false,
		errors:               errs,
		receivedNonceHistory: make(map[int]time.Time),
		sentNonceHistory:     make(map[int]time.Time),
	}

	// Connect message containing this end's public key
	logger.Debug("Exchanging public keys", "StreamID", id)
	keyExchangeReceive, keyExchangeSend := otw.New[rsa.PublicKey]().
		UseJSONEncoding().
		UseNonce(msgStream.newNonce, msgStream.checkReceivedNonce).
		UseCompression().
		UseTimeout(time.Second * 15).
		Build()

	if _, err := keyExchangeSend(*msgStream.pubKey, sender); err != nil {
		logger.Error("Failed to send public key during public key exchange", "Error", err)
		return nil, err
	}

	// Receive target's public key
	logger.Debug("Waiting for public key from client...", "StreamID", id)
	receivedPubKey, _, err := keyExchangeReceive(receiver)
	if err != nil {
		logger.Error("Failed to recieve public key during public key exchange", "Error", err)
		return nil, err
	}

	logger.Debug("Received public key from client", "StreamID", id)
	msgStream.tgtPubKey = &receivedPubKey

	rmf, smf := otw.New[Message]().
		UseJSONEncoding().
		UseNonce(msgStream.newNonce, msgStream.checkReceivedNonce).
		UseAsymmetricEncryption(func() *rsa.PublicKey {
			return msgStream.tgtPubKey
		}, func() *rsa.PrivateKey {
			return privKey
		}).
		UseCompression().
		UseTimeout(time.Second * 15).
		Build()

	msgStream.readMsgFn = rmf
	msgStream.sendMsgFn = smf

	go func() {
		logger.Debug("Waiting for Messages from client...", "StreamID", id)
		for !msgStream.closed {
			msg, err := msgStream.receiveMessage()
			if err != nil {
				msgStream.errors <- err
				continue
			}
			logger.Info("Received Message", "Type", msg.Type, "StreamID", id)
			output <- msg
		}
	}()

	logger.Info("Message Stream successfully negotiated", "StreamID", id)
	return msgStream, nil
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
	ms.closed = true
	close(ms.output)
}

// Sends a Message on the io.Writer portion of the Message Stream.
//
// Returns an error if it fails serialise the metadata or payload, write data to the underlying `io.Writer` or generate a nonce.
func (ms *MessageStream) SendMessage(t MessageType, metadata map[string]any, payload any) error {
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
	if _, err := ms.sendMsgFn(*m, ms.sender); err != nil {
		return err
	}
	return nil
}

func (ms *MessageStream) receiveMessage() (*Message, error) {
	logger.Debug("Beginning Message receive...", "StreamID", ms.id)
	msg, _, err := ms.readMsgFn(ms.receiver)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}
