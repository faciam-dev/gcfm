package crypto

import pcrypto "github.com/faciam-dev/gcfm/pkg/crypto"

// Encrypt encrypts plaintext using repository's crypto package.
func Encrypt(b []byte) ([]byte, error) {
	return pcrypto.Encrypt(b)
}

// Decrypt decrypts ciphertext using repository's crypto package.
func Decrypt(b []byte) ([]byte, error) {
	return pcrypto.Decrypt(b)
}

// CheckEnv validates that encryption key is set.
func CheckEnv() error {
	return pcrypto.CheckEnv()
}
