package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"go-api/internal/domain/port"
)

const cipherPrefix = "v1:"

// AESCipher encrypts secret values at rest with AES-256-GCM. The key is 32
// bytes, base64 encoded (SECRETS_ENCRYPTION_KEY).
type AESCipher struct {
	aead cipher.AEAD
}

func NewAESCipher(base64Key string) (*AESCipher, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESCipher{aead: aead}, nil
}

var _ port.SecretCipher = (*AESCipher)(nil)

func (c *AESCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return cipherPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *AESCipher) Decrypt(ciphertext string) (string, error) {
	if len(ciphertext) < len(cipherPrefix) || ciphertext[:len(cipherPrefix)] != cipherPrefix {
		return "", errors.New("unsupported ciphertext format")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext[len(cipherPrefix):])
	if err != nil {
		return "", err
	}
	size := c.aead.NonceSize()
	if len(raw) < size {
		return "", errors.New("ciphertext too short")
	}
	plain, err := c.aead.Open(nil, raw[:size], raw[size:], nil)
	if err != nil {
		return "", errors.New("cannot decrypt secret")
	}
	return string(plain), nil
}
