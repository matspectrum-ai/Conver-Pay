package secretbox

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

const versionPrefix = "v1."

var ErrInvalidCiphertext = errors.New("invalid ciphertext")

type Box struct {
	aead cipher.AEAD
}

func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Box{aead: aead}, nil
}

func NewBase64(encoded string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode secretbox key: %w", err)
	}
	return New(key)
}

func (b *Box) Encrypt(plaintext []byte) (string, error) {
	if b == nil || b.aead == nil {
		return "", fmt.Errorf("secretbox is not configured")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := b.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return versionPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (b *Box) Decrypt(ciphertext string) ([]byte, error) {
	if b == nil || b.aead == nil {
		return nil, fmt.Errorf("secretbox is not configured")
	}
	if !strings.HasPrefix(ciphertext, versionPrefix) {
		return nil, ErrInvalidCiphertext
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, versionPrefix))
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	if len(payload) < b.aead.NonceSize() {
		return nil, ErrInvalidCiphertext
	}
	nonce := payload[:b.aead.NonceSize()]
	sealed := payload[b.aead.NonceSize():]
	plaintext, err := b.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}
