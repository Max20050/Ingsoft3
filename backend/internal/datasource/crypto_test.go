package datasource

import (
	"encoding/base64"
	"testing"
)

func testKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := NewEncryptor(testKey())
	if err != nil {
		t.Fatalf("NewEncryptor returned error: %v", err)
	}

	ciphertext, err := enc.Encrypt("s3cret-password")
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if plaintext != "s3cret-password" {
		t.Errorf("got plaintext %q, want %q", plaintext, "s3cret-password")
	}
}

func TestNewEncryptorRejectsWrongKeyLength(t *testing.T) {
	shortKey := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := NewEncryptor(shortKey); err == nil {
		t.Error("expected error for a key that isn't 32 bytes, got nil")
	}
}

func TestNewEncryptorRejectsInvalidBase64(t *testing.T) {
	if _, err := NewEncryptor("not-valid-base64!!"); err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}
