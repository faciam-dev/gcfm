package auth

import (
	"testing"
	"time"
)

func TestGenerateWithTenant(t *testing.T) {
	j := NewJWT("secret", time.Minute)
	tok, err := j.GenerateWithTenant(1, "t1")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := j.Validate(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.ID != "t1" {
		t.Fatalf("tenant id=%s", claims.ID)
	}
}
