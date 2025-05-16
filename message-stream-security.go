package messagestream

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

func generateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey) {
	// This error is ignored for now
	// Documentation says that anything < 1024 will cause an error and we will use a hardcoded value for now
	// So unless a read error can occur withing rand.Reader, I'm ignoring this for now
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubKey := privKey.PublicKey

	return privKey, &pubKey
}

func pemEncode(pubKey *rsa.PublicKey) ([]byte, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, err
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return pemBytes, nil
}

func pemDecode(raw []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		return nil, fmt.Errorf("failed to parse public key")
	}

	rawKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse x509 public key: %s", err.Error())
	}

	pubKey, ok := rawKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return pubKey, nil
}

func encrypt(payload []byte, pubKey *rsa.PublicKey) []byte {
	encrypted, _ := rsa.EncryptPKCS1v15(rand.Reader, pubKey, payload)
	return encrypted
}

func decrypt(encrypted []byte, privKey *rsa.PrivateKey) []byte {
	decrypted, _ := rsa.DecryptPKCS1v15(rand.Reader, privKey, encrypted)
	return decrypted
}

func sign(payload []byte, privKey *rsa.PrivateKey) []byte {
	hash := sha256.Sum256(payload)
	signature, _ := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	return signature
}

func isValid(payload, signature []byte, pubKey *rsa.PublicKey) bool {
	hash := sha256.Sum256(payload)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], signature) == nil
}
