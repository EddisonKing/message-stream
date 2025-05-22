package messagestream

import (
	"crypto/rand"
	"crypto/rsa"
)

func generateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey) {
	logger.Debug("Generating RSA private/public key pair")
	// This error is ignored for now
	// Documentation says that anything < 1024 will cause an error and we will use a hardcoded value for now
	// So unless a read error can occur withing rand.Reader, I'm ignoring this for now
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubKey := privKey.PublicKey

	logger.Debug("RSA key pair generated")
	return privKey, &pubKey
}
