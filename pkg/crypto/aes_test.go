package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	t.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")
	plain := []byte("secret dsn")
	enc, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatalf("round trip mismatch: %q != %q", dec, plain)
	}
}

func TestCheckEnvMissing(t *testing.T) {
	t.Setenv("CF_ENC_KEY", "")
	if err := CheckEnv(); err == nil {
		t.Fatal("expected error when key missing")
	}
}
