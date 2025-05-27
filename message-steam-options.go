package messagestream

import (
	"crypto/rsa"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
)

// Configuration options for creating and running a Message Stream
type MessageStreamOptions struct {
	// Enables the use of RSA keys to perform Message encryption
	UseAsymmetricEncryption bool

	// RSA Private Key used for signing Messages and decrypting incoming Messages
	PrivateKey *rsa.PrivateKey

	// RSA Public Key to be sent to the other side of the Stream to be used for encrypting Messages that only this side of the stream can decrypt
	PublicKey *rsa.PublicKey

	// An identifier for the Message Stream, intended to be used for Logging correlation. It is not use in the communications protocol
	ID string

	// A structured logger for logging the loggy things
	Logger *slog.Logger

	// A timeout for sending and receiving public keys during key exchange
	KeyExchangeTimeout time.Duration

	// A timeout for sending and receiving Messages post key exchange
	MessageExchangeTimeout time.Duration

	// Forwards the given logger into underlying libraries and structures to produce even more loggy things for additional logginess
	DeepLogging bool

	// Enable Message Proxying
	AllowProxying bool
}

// Creates a new configuration with default values for the Message Stream. This will be used if `nil` is passed during the construction of a new Message Stream
func NewMessageStreamOptions() *MessageStreamOptions {
	privKey, pubKey := GenerateRSAKeyPair()
	return &MessageStreamOptions{
		UseAsymmetricEncryption: true,
		PrivateKey:              privKey,
		PublicKey:               pubKey,
		ID:                      uuid.NewString(),
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})),
		KeyExchangeTimeout:     time.Second * 60,
		MessageExchangeTimeout: time.Second * 60,
		DeepLogging:            false,
		AllowProxying:          false,
	}
}
