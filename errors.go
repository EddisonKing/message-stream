package messagestream

import "errors"

var (
	ErrFailedToSendPublicKey      = errors.New("failed to send public key during public key exchange")
	ErrFailedToReceivePublicKey   = errors.New("failed to receive public key during public key exchange")
	ErrProxyAddressUnresolvable   = errors.New("failed to resolve proxy address")
	ErrFailedToUnwrapProxyMessage = errors.New("failed to unwrap proxied Message")
	ErrMalformedProxies           = errors.New("proxy addresses are malformed")
	ErrMessageStreamClosed        = errors.New("message Stream is closed")
	ErrMissingPublicKey           = errors.New("missing public key")
	ErrMissingPrivateKey          = errors.New("missing private key")
	ErrProxyingNotAllowed         = errors.New("proxying is not allowed")
	ErrFailedToProxyMessage       = errors.New("failed to proxy Message")
	ErrFailedToProxyReplyMessage  = errors.New("failed to return Message from proxy")
)
