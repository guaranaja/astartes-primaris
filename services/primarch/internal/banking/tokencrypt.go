package banking

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

// TokenCrypter encrypts/decrypts bank access tokens with AES-256-GCM using a
// key from the PLAID_TOKEN_ENC_KEY env var (base64, 32 bytes decoded).
// This is a pragmatic on-DB encryption layer; upgrade to GCP Secret Manager
// (one secret per connection) if blast radius grows.
type TokenCrypter struct {
	aead cipher.AEAD
}

// NewTokenCrypter reads PLAID_TOKEN_ENC_KEY and returns a crypter, or nil + err
// if the key is missing or malformed. Callers decide how to behave when nil
// (typically: disable banking until the key is set).
func NewTokenCrypter() (*TokenCrypter, error) {
	raw := os.Getenv("PLAID_TOKEN_ENC_KEY")
	if raw == "" {
		return nil, errors.New("PLAID_TOKEN_ENC_KEY not set")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// Allow hex-encoded 64-char keys too, but base64 is the documented form
		return nil, fmt.Errorf("PLAID_TOKEN_ENC_KEY not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("PLAID_TOKEN_ENC_KEY must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	return &TokenCrypter{aead: aead}, nil
}

// Encrypt produces a base64 string safe to persist. Format: base64(nonce || ciphertext).
func (c *TokenCrypter) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ct := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	buf := make([]byte, 0, len(nonce)+len(ct))
	buf = append(buf, nonce...)
	buf = append(buf, ct...)
	return base64.StdEncoding.EncodeToString(buf), nil
}

// Decrypt reverses Encrypt.
func (c *TokenCrypter) Decrypt(encoded string) (string, error) {
	buf, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(buf) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := buf[:ns], buf[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}

// GenerateKey is a helper for bootstrapping: call once to produce a base64 key
// you paste into GCP Secret Manager. Not used at runtime; only documentation.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
