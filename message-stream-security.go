package messagestream

import (
	"crypto/rand"
	"crypto/rsa"
)

// Convenience method to generate an RSA Private and Public key pair with a bit size of 2048.
//
// If the default options, such as 2048 bit key size is suitable, this method is ok to use. But it is recommended, for security,
// to create the RSA keys on your own and take ownership of how they are built.
func GenerateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey) {
	// This error is ignored for now
	// Documentation says that anything < 1024 will cause an error and we will use a hardcoded value for now
	// So unless a read error can occur withing rand.Reader, I'm ignoring this for now
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	return privKey, &privKey.PublicKey
}
