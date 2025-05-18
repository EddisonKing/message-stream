package messagestream

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
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

func pemEncode(pubKey *rsa.PublicKey) ([]byte, error) {
	logger.Debug("Encoding public key")
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, err
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	logger.Debug("Public key encoded")
	return pemBytes, nil
}

func pemDecode(raw []byte) (*rsa.PublicKey, error) {
	logger.Debug("Decoding PEM")

	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		logger.Error("Failed to parse public key")
		return nil, fmt.Errorf("failed to parse public key")
	}

	rawKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logger.Error("Failed to parse x509 public key", "Error", err)
		return nil, fmt.Errorf("failed to parse x509 public key: %s", err.Error())
	}

	pubKey, ok := rawKey.(*rsa.PublicKey)
	if !ok {
		logger.Error("Key was not an RSA public key")
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	logger.Debug("PEM Decoded")
	return pubKey, nil
}

func encrypt(payload []byte, pubKey *rsa.PublicKey) []byte {
	logger.Debug("Encrypting payload", "PayloadSize", len(payload))
	encrypted, _ := rsa.EncryptPKCS1v15(rand.Reader, pubKey, payload)
	logger.Debug("Payload encrypted", "EncryptedPayloadSize", len(encrypted))
	return encrypted
}

func decrypt(encrypted []byte, privKey *rsa.PrivateKey) []byte {
	logger.Debug("Decrypting payload", "EncryptedPayloadSize", len(encrypted))
	decrypted, _ := rsa.DecryptPKCS1v15(rand.Reader, privKey, encrypted)
	logger.Debug("Payload decrypted", "PayloadSize", len(decrypted))
	return decrypted
}

func sign(payload []byte, privKey *rsa.PrivateKey) []byte {
	logger.Debug("Signing payload")
	hash := sha256.Sum256(payload)
	logger.Debug("Payload hashed", "PayloadHash", hex.EncodeToString(hash[:]))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	logger.Debug("Payload signed", "Signature", hex.EncodeToString(signature))
	return signature
}

func isValid(payload, signature []byte, pubKey *rsa.PublicKey) bool {
	logger.Debug("Validating Signature", "Signature", hex.EncodeToString(signature[:]))
	hash := sha256.Sum256(payload)
	logger.Debug("Payload hashed", "PayloadHash", hex.EncodeToString(hash[:]))
	defer logger.Debug("Signature Validated")
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], signature) == nil
}
