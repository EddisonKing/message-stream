package messagestream

import (
	"bytes"
	"crypto/rsa"
	"encoding/binary"
	"encoding/json"
)

func intToBytes(i int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(i))
	return b
}

func bytesToInt(b []byte) int {
	return int(binary.BigEndian.Uint32(b))
}

// Represents a Message Type. Ultimately, just a string.
type MessageType string

// Represents a Message that can be sent on a Message Stream. Most likely, you want
// NewMessage() so you can encode the metadata and payload properly.
type Message struct {
	Type     MessageType
	metadata map[string]any
	payload  []byte
}

func (m *Message) bytes() ([]byte, error) {
	if m == nil {
		return []byte{}, nil
	}

	serialised := bytes.NewBuffer(nil)

	typeBytes := m.serialiseType()
	metadataBytes, err := m.serialiseMetadata()
	if err != nil {
		return nil, err
	}

	writeLV(typeBytes, serialised)
	writeLV(metadataBytes, serialised)
	writeLV(m.payload, serialised)

	return serialised.Bytes(), nil
}

func (m *Message) serialiseType() []byte {
	b := []byte(string(m.Type))
	return b
}

func (m *Message) serialiseMetadata() ([]byte, error) {
	return json.Marshal(m.metadata)
}

// Create a new Message to be sent via a Message Stream.
// The message will be encrypted by the Message Stream when sent using the recipient's public key.
func newMessage(t MessageType, metadata map[string]any, payload any) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:     t,
		metadata: metadata,
		payload:  payloadBytes,
	}, nil
}

// Returns the payload as the type P. For example, if a struct called Payload is sent,
// then ExtractPayload[Payload](someMessage) will return an instance of the Payload struct
// that was sent. Attempts to deserialize into an incorrect type will result in an error.
func ExtractPayload[P any](msg *Message) (P, error) {
	var payload P
	if err := json.Unmarshal(msg.payload, &payload); err != nil {
		return *new(P), err
	}
	return payload, nil
}

// Returns the metadata attached to a Message
func ExtractMetadata(msg *Message) map[string]any {
	return msg.metadata
}

const magicStr = "!~:"

type headerFlag byte

const (
	headerContainsPublicKey headerFlag = 0x00000001 << iota
	headerEncryptedWithRecipientPublicKey
)

type messageHeader struct {
	flags       headerFlag
	nonce       int
	pubKeyBytes []byte
	tgtPubKey   *rsa.PublicKey
}

func createMessageHeader(flags headerFlag, nonce int, pubKeyBytes []byte, tgtPubKey *rsa.PublicKey) *messageHeader {
	return &messageHeader{
		flags:       flags,
		nonce:       nonce,
		pubKeyBytes: pubKeyBytes,
		tgtPubKey:   tgtPubKey,
	}
}

func (m *messageHeader) bytes() []byte {
	b := bytes.NewBuffer(nil)

	writeV([]byte(magicStr), b)

	writeV([]byte{byte(m.flags)}, b)

	if m.pubKeyBytes != nil && (m.flags.hasHeader(headerContainsPublicKey)) {
		writeLV(m.pubKeyBytes, b)
	}

	if m.flags.hasHeader(headerEncryptedWithRecipientPublicKey) && m.tgtPubKey != nil {
		encryptedNonceBytes := encrypt(intToBytes(m.nonce), m.tgtPubKey)
		writeLV(encryptedNonceBytes, b)
	} else {
		nonceBytes := intToBytes(m.nonce)
		writeLV(nonceBytes, b)
	}

	return b.Bytes()
}

func (h headerFlag) hasHeader(b headerFlag) bool {
	return byte(b)&byte(h) == byte(h)
}
