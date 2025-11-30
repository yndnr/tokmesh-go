package persistence

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

type dataCipher interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

type encryptedEnvelope struct {
	Enc string `json:"enc"`
}

type aesGCMCipher struct {
	gcm cipher.AEAD
}

func newAESCipher(key []byte) (dataCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes-gcm: %w", err)
	}
	return &aesGCMCipher{gcm: gcm}, nil
}

func (c *aesGCMCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	out := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return out, nil
}

func (c *aesGCMCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	size := c.gcm.NonceSize()
	if len(ciphertext) < size {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:size]
	data := ciphertext[size:]
	plain, err := c.gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}
