package cli_test

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHashVerify(t *testing.T) {
	pw := "secret"
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if bcrypt.CompareHashAndPassword(h, []byte(pw)) != nil {
		t.Fatalf("verify failed")
	}
}
