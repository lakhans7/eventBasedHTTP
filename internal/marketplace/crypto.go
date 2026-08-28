package marketplace

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// TokenCipher encrypts marketplace OAuth tokens at rest with AES-256-GCM.
// Nothing calls this today (see docs/fiverr-api-capabilities.md — there is no
// Fiverr token to store), but it exists and is unit-tested now so that
// storing a real token later, if Fiverr or another marketplace ever ships
// OAuth, is a one-line call rather than a new security feature to design
// under time pressure.
type TokenCipher struct {
	gcm cipher.AEAD
}

func NewTokenCipher(key []byte) (*TokenCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("token encryption key must be exactly 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &TokenCipher{gcm: gcm}, nil
}

func (t *TokenCipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, t.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return t.gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (t *TokenCipher) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := t.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := t.gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
