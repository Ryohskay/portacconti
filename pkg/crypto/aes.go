package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns nonce || ciphertext.
func Encrypt(keyHex string, plaintext []byte) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes (64 hex chars)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts nonce || ciphertext produced by Encrypt.
func Decrypt(keyHex string, data []byte) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes (64 hex chars)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptString is a convenience wrapper for string values.
func EncryptString(keyHex, plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	return Encrypt(keyHex, []byte(plaintext))
}

// DecryptString is a convenience wrapper for string values.
func DecryptString(keyHex string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	b, err := Decrypt(keyHex, data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
