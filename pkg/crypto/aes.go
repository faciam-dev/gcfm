package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

func keyBytes() ([]byte, error) {
	k := os.Getenv("CF_ENC_KEY")
	if len(k) == 0 {
		return nil, fmt.Errorf("CF_ENC_KEY not set")
	}
	b := []byte(k)
	if l := len(b); l != 16 && l != 24 && l != 32 {
		return nil, fmt.Errorf("invalid key length %d", l)
	}
	return b, nil
}

// Encrypt encrypts plaintext using AES-GCM with key from CF_ENC_KEY.
func Encrypt(plain []byte) ([]byte, error) {
	key, err := keyBytes()
	if err != nil {
		return nil, err
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
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// Decrypt decrypts ciphertext using AES-GCM with key from CF_ENC_KEY.
func Decrypt(ciphertext []byte) ([]byte, error) {
	key, err := keyBytes()
	if err != nil {
		return nil, err
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
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
