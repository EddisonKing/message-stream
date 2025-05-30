package messagestream

import (
	"encoding/json"
)

// Represents a Message Type.
type MessageType string

const (
	msxProxy = MessageType("ms-x-proxy")
)

// Represents a Message that can be sent on a Message Stream. Most likely, you want
// NewMessage() so you can encode the metadata and payload properly.
type Message struct {
	Type     MessageType
	Metadata map[string]any
	Payload  []byte
	Proxies  []string
}

// Create a new Message to be sent via a Message Stream.
// The message will be encrypted by the Message Stream when sent using the recipient's public key.
func newMessage(t MessageType, metadata map[string]any, payload any, proxies ...string) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:     t,
		Metadata: metadata,
		Payload:  payloadBytes,
		Proxies:  proxies,
	}, nil
}

// Unwraps the Message into it's parts, the payload and the metadata.
// Returns the payload as the type P. For example, if a struct called Payload is sent,
// then Unwrap[Payload](someMessage) will return an instance of the Payload struct
// that was sent. Attempts to deserialize into an incorrect type will result in an error.
func Unwrap[P any](msg *Message) (P, map[string]any, error) {
	var payload P
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return *new(P), nil, err
	}

	return payload, msg.Metadata, nil
}
