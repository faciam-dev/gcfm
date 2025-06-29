package auth_test

import (
	"testing"
	"time"

	"github.com/faciam-dev/gcfm/internal/auth"
)

func TestJWTGenerateValidate(t *testing.T) {
	j := auth.NewJWT("secret", time.Minute)
	tok, err := j.Generate(42)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := j.Validate(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.Subject != "42" {
		t.Fatalf("subject mismatch: %s", claims.Subject)
	}
}
