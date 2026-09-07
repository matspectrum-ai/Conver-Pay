package secretbox

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestBoxRoundTripAndWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	box, err := New(key)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ciphertext, err := box.Encrypt([]byte("merchant-signing-secret-0123456789"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if !strings.HasPrefix(ciphertext, "v1.") || strings.Contains(ciphertext, "merchant-signing") {
		t.Fatalf("unexpected ciphertext = %q", ciphertext)
	}
	plaintext, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(plaintext) != "merchant-signing-secret-0123456789" {
		t.Fatalf("plaintext = %q", plaintext)
	}

	other, _ := New(bytes.Repeat([]byte{0x22}, 32))
	if _, err := other.Decrypt(ciphertext); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("wrong-key error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestNewBase64Requires32Bytes(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32))
	if _, err := NewBase64(encoded); err != nil {
		t.Fatalf("NewBase64() error = %v", err)
	}
	if _, err := NewBase64(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("NewBase64() error = nil, want validation error")
	}
}
