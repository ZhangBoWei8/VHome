package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrSecretEncryptionUnavailable = errors.New("secret encryption key is not configured")

// EncryptionKey is the base64-encoded AES-256 key. It is a named type so that
// dependency injection can tell it apart from every other string in the graph.
type EncryptionKey string

// SecretBox encrypts credentials with AES-256-GCM. A random nonce is stored
// in front of every ciphertext, so saving the same password twice still
// produces different database values.
type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(key EncryptionKey) (*SecretBox, error) {
	encodedKey := strings.TrimSpace(string(key))
	if encodedKey == "" {
		return &SecretBox{}, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode secret encryption key: %w", err)
	}
	if len(decoded) != 32 {
		return nil, errors.New("secret encryption key must decode to exactly 32 bytes")
	}

	block, err := aes.NewCipher(decoded)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret GCM: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

func (b *SecretBox) Available() bool {
	return b != nil && b.aead != nil
}

func (b *SecretBox) Encrypt(plaintext string) ([]byte, error) {
	if !b.Available() {
		return nil, ErrSecretEncryptionUnavailable
	}
	if plaintext == "" {
		return nil, errors.New("cannot encrypt an empty secret")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate secret nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (b *SecretBox) Decrypt(ciphertext []byte) (string, error) {
	if !b.Available() {
		return "", ErrSecretEncryptionUnavailable
	}
	if len(ciphertext) <= b.aead.NonceSize() {
		return "", errors.New("encrypted secret is malformed")
	}
	nonce := ciphertext[:b.aead.NonceSize()]
	payload := ciphertext[b.aead.NonceSize():]
	plaintext, err := b.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}
