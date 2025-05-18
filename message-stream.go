package messagestream

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"
)

// Message Stream supporting Send and Receive operations.
type MessageStream struct {
	id                   uuid.UUID
	tgtPubKey            *rsa.PublicKey
	pubKey               *rsa.PublicKey
	pubKeyBytes          []byte
	privKey              *rsa.PrivateKey
	sender               io.Writer
	receiver             io.Reader
	output               chan *Message
	errors               chan error
	closed               bool
	receivedNonceHistory map[int]time.Time
	sentNonceHistory     map[int]time.Time
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
	id := uuid.New()

	logger.Info("Creating a new Message Stream", "StreamID", id)
	privKey, pubKey := generateRSAKeyPair()
	pubKeyBytes, err := pemEncode(pubKey)
	if err != nil {
		return nil, err
	}

	output := make(chan *Message, 50)
	errs := make(chan error, 30)

	msgStream := &MessageStream{
		id:                   id,
		pubKey:               pubKey,
		pubKeyBytes:          pubKeyBytes,
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
	logger.Debug("Beginning public key exchange", "StreamID", id)
	nonce, err := msgStream.generateNonce()
	if err != nil {
		return nil, err
	}
	msgStream.storeNonce(&msgStream.sentNonceHistory, nonce)

	logger.Debug("Sending public key", "StreamID", id)
	msgStream.sendMessage(createMessageHeader(headerContainsPublicKey, nonce, msgStream.pubKeyBytes, nil), nil)

	// Receive target's public key
	logger.Debug("Waiting for public key from client...", "StreamID", id)
	header, _, _ := msgStream.receiveMessage()

	logger.Debug("Received public key from client", "StreamID", id)
	msgStream.tgtPubKey = header.tgtPubKey

	// Consciously ignoring error here as it is impossible at this point to see a repeated nonce
	msgStream.storeNonce(&msgStream.receivedNonceHistory, header.nonce)

	go func() {
		logger.Debug("Waiting for Messages from client...", "StreamID", id)
		for !msgStream.closed {
			_, msg, signature := msgStream.receiveMessage()
			logger.Info("Received Message", "Type", msg.Type, "StreamID", id)

			msgBytes, err := msg.bytes()
			if err != nil {
				logger.Error("Message failed to serialise", "Error", err, "StreamID", id)
				errs <- err
				continue
			}

			logger.Debug("Checking received Message's signature", "StreamID", id)
			if isValid(msgBytes, signature, msgStream.tgtPubKey) {
				logger.Debug("Message succeeded validation", "StreamID", id)
				output <- msg
			} else {
				logger.Warn("Message did not contain a valid signature", "StreamID", id)
			}
		}
	}()

	logger.Info("Message Stream successfully negotiated", "StreamID", id)
	return msgStream, nil
}

func (ms *MessageStream) generateNonce() (int, error) {
	nonce, err := rand.Int(rand.Reader, big.NewInt(int64(math.MaxInt32)))
	if err != nil {
		logger.Error("Failed to generate nonce", "Error", err, "StreamID", ms.id)
		return 0, err
	}

	logger.Debug("Generated new nonce", "Nonce", nonce.Int64(), "StreamID", ms.id)
	return int(nonce.Int64()), nil
}

func (ms *MessageStream) newNonce() (int, error) {
	var n int
	for {
		nonce, err := ms.generateNonce()
		if err != nil {
			return 0, err
		}

		if err = ms.storeNonce(&ms.sentNonceHistory, nonce); err != nil {
			continue
		}

		n = nonce
		break
	}
	return n, nil
}

const nonceTTL time.Duration = time.Minute * 15 // Not sure about the time, this should be relative to the time it might take to randomly generate the same nonce again

func (ms *MessageStream) storeNonce(nonceHistory *map[int]time.Time, nonce int) error {
	if seenTime, exists := (*nonceHistory)[nonce]; exists {
		if time.Since(seenTime) < nonceTTL {
			logger.Warn("Nonce already stored", "Nonce", nonce, "StreamID", ms.id)
			return fmt.Errorf("nonce %d has already been received, this could be a replay", nonce, "StreamID", ms.id)
		}
	}
	// Refresh or set nonce TTL
	logger.Debug("Storing new sent nonce", "Nonce", nonce, "StreamID", ms.id)
	(*nonceHistory)[nonce] = time.Now().UTC()
	return nil
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
	logger.Info("Sending Message", "StreamID", ms.id)
	m, err := newMessage(t, metadata, payload)
	if err != nil {
		return err
	}

	logger.Debug("Message created", "Type", t, "StreamID", ms.id)

	n, err := ms.newNonce()
	if err != nil {
		return err
	}

	return ms.sendMessage(createMessageHeader(0, n, nil, ms.tgtPubKey), m)
}

// Forward an existing Message. This is useful in a situation where multiple Message Streams are being used and a received Message needs to be passed to a different Message Stream.
//
// Returns an error if it fails to write data to the underlying `io.Writer` or generate a nonce.
func (ms *MessageStream) ForwardMessage(msg *Message) error {
	logger.Info("Forwarding Message", "Type", msg.Type, "StreamID", ms.id)

	n, err := ms.newNonce()
	if err != nil {
		return err
	}

	return ms.sendMessage(createMessageHeader(0, n, nil, ms.tgtPubKey), msg)
}

// Returns a channel where incoming Messages can be received.
func (ms *MessageStream) Receiver() <-chan *Message {
	return ms.output
}

// Returns a channel where any errors generated by the Message Stream's operations will be sent.
func (ms *MessageStream) Errors() <-chan error {
	return ms.errors
}

func (ms *MessageStream) sendMessage(h *messageHeader, m *Message) error {
	logger.Debug("Provisining new buffer to send Message", "StreamID", ms.id)

	b := bytes.NewBuffer(nil)

	headerBytes := h.bytes()
	b.Write(headerBytes)
	logger.Debug("Wrote Message header to buffer", "Size", len(headerBytes), "StreamID", ms.id)

	msgBytes, err := m.bytes()
	if err != nil {
		ms.errors <- err
		return nil
	}

	if ms.tgtPubKey != nil {
		logger.Debug("Target public key is set, using encryption", "StreamID", ms.id)

		encryptedBytes := encrypt(msgBytes, ms.tgtPubKey)
		writeLV(encryptedBytes, b)
		logger.Debug("Wrote encrypted Message to buffer", "Size", len(encryptedBytes), "StreamID", ms.id)

		signature := sign(msgBytes, ms.privKey)
		writeLV(signature, b)
		logger.Debug("Wrote Message signature to buffer", "Size", len(signature), "StreamID", ms.id)
	} else {
		logger.Debug("Target public key is not set, sending unencrypted", "StreamID", ms.id)
		writeLV(msgBytes, b)
		logger.Debug("Wrote unencrypted Message to buffer", "Size", len(msgBytes), "StreamID", ms.id)
	}

	n, err := ms.sender.Write(b.Bytes())
	if err != nil {
		ms.errors <- err
		logger.Error("Failed to write buffer to Message Stream writer", "Error", err, "StreamID", ms.id)
		return err
	}
	logger.Debug("Wrote buffer to Message Stream writer", "Size", n, "StreamID", ms.id)

	return nil
}

func (ms *MessageStream) reportIfError(data []byte, err error) ([]byte, bool) {
	if err != nil {
		if err != io.EOF {
			ms.errors <- err
		}
		return nil, false
	}
	return data, true
}

func (ms *MessageStream) receiveMessage() (*messageHeader, *Message, []byte) {
	logger.Debug("Beginning Message receive...", "StreamID", ms.id)
	for {
		logger.Debug("Reading Magic Bytes...", "StreamID", ms.id)
		magicBytes, ok := ms.reportIfError(readV(3, ms.receiver))
		if !ok {
			continue
		}

		if string(magicBytes) != magicStr {
			logger.Warn("Magic Bytes were not read, discarding and continuing...", "Bytes", magicBytes, "StreamID", ms.id)
			continue
		}
		logger.Debug("Read Magic Bytes", "Bytes", magicBytes, "StreamID", ms.id)

		logger.Debug("Reading Message flags...", "StreamID", ms.id)
		flagBytes, ok := ms.reportIfError(readV(1, ms.receiver))
		if !ok {
			continue
		}

		flags := headerFlag(flagBytes[0])
		logger.Debug("Read Message flags", "Flags", fmt.Sprintf("%08b", byte(flags)), "StreamID", ms.id)

		var tgtPubKey *rsa.PublicKey
		if flags&headerContainsPublicKey == 1 {
			logger.Debug("Reading public key bytes...", "StreamID", ms.id)
			tgtPubKeyBytes, ok := ms.reportIfError(readLV(4, ms.receiver))
			if !ok {
				continue
			}
			logger.Debug("Read public key bytes", "Size", len(tgtPubKeyBytes), "StreamID", ms.id)

			pubKey, err := pemDecode(tgtPubKeyBytes)
			if err != nil {
				ms.errors <- err
				continue
			}
			tgtPubKey = pubKey
		}

		logger.Debug("Reading nonce...", "StreamID", ms.id)
		nonceBytes, ok := ms.reportIfError(readLV(4, ms.receiver))
		if !ok {
			continue
		}

		if flags.hasHeader(headerEncryptedWithRecipientPublicKey) {
			nonceBytes = decrypt(nonceBytes, ms.privKey)
		}

		nonce := bytesToInt(nonceBytes)
		logger.Debug("Read nonce", "Nonce", nonce, "StreamID", ms.id)

		// Abandoning any messages with a repeated nonce as it may be a replay attack
		if err := ms.storeNonce(&ms.receivedNonceHistory, nonce); err != nil {
			ms.errors <- err
			logger.Error("Received nonce has been seen before. Potential replay attack. Abandoning message.", "Nonce", nonce, "StreamID", ms.id)
			return nil, nil, nil
		}

		header := &messageHeader{
			tgtPubKey: tgtPubKey,
			nonce:     nonce,
		}

		logger.Debug("Reading Message body...", "StreamID", ms.id)
		messageBytes, ok := ms.reportIfError(readLV(4, ms.receiver))
		if !ok || len(messageBytes) == 0 {
			logger.Debug("Message body was empty", "StreamID", ms.id)
			return header, nil, nil
		}

		if flags.hasHeader(headerEncryptedWithRecipientPublicKey) {
			messageBytes = decrypt(messageBytes, ms.privKey)
		}

		messageBuffer := bytes.NewReader(messageBytes)

		logger.Debug("Reading Message Type...", "StreamID", ms.id)
		msgTypeBytes, ok := ms.reportIfError(readLV(4, messageBuffer))
		if !ok {
			continue
		}
		msgType := MessageType(string(msgTypeBytes))

		logger.Debug("Read Message Type", "Type", msgType, "StreamID", ms.id)

		logger.Debug("Reading metadata bytes...", "StreamID", ms.id)
		metadataBytes, ok := ms.reportIfError(readLV(4, messageBuffer))
		if !ok {
			continue
		}

		logger.Debug("Read metadata bytes", "Size", len(metadataBytes), "StreamID", ms.id)
		var metadata map[string]any
		err := json.Unmarshal(metadataBytes, &metadata)
		if err != nil {
			ms.errors <- err
			logger.Error("Failed to deserialise metadata bytes into metadata map", "Error", err, "StreamID", ms.id)
		}

		logger.Debug("Reading payload bytes...", "StreamID", ms.id)
		payloadBytes, ok := ms.reportIfError(readLV(4, messageBuffer))
		if !ok {
			continue
		}
		logger.Debug("Read payload bytes", "Size", len(payloadBytes), "StreamID", ms.id)

		logger.Debug("Reading signature bytes...", "StreamID", ms.id)
		signature, ok := ms.reportIfError(readLV(4, ms.receiver))
		if !ok {
			continue
		}
		logger.Debug("Read signature bytes", "Size", len(metadataBytes), "StreamID", ms.id)

		logger.Debug("Full message received", "Type", msgType, "StreamID", ms.id)
		return header, &Message{
			metadata: metadata,
			Type:     msgType,
			payload:  payloadBytes,
		}, signature
	}
}
