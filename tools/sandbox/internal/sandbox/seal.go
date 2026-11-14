package sandbox

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

type sealKey struct {
	private *rsa.PrivateKey
}

func newSealKey() (sealKey, error) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return sealKey{}, fmt.Errorf("generate the seal key: %w", err)
	}
	return sealKey{private: private}, nil
}

func (k sealKey) Public() (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&k.private.PublicKey)
	if err != nil {
		return "", fmt.Errorf("encode the seal key: %w", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return base64.StdEncoding.EncodeToString(block), nil
}

func (k sealKey) Open(sealed string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("decode the sealed tunnel address: %w", err)
	}
	plaintext, err := rsa.DecryptOAEP(sha256.New(), nil, k.private, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("open the sealed tunnel address: %w", err)
	}
	return string(plaintext), nil
}
