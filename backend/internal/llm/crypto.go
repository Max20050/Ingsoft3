package llm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encryptor encrypts and decrypts LLM API keys at rest using AES-256-GCM.
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor builds an Encryptor from a base64-encoded 32-byte AES-256 key.
func NewEncryptor(base64Key string) (*Encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must decode to 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encryptor{gcm: gcm}, nil
}

// Encrypt seals plaintext, prefixing the result with a random nonce.
func (e *Encryptor) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return e.gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt opens ciphertext produced by Encrypt.
func (e *Encryptor) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
