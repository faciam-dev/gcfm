package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT handles token generation and validation.
type JWT struct {
	secret []byte
	exp    time.Duration
}

// NewJWT returns a new JWT handler.
func NewJWT(secret string, exp time.Duration) *JWT {
	return &JWT{secret: []byte(secret), exp: exp}
}

// Generate creates a signed token for the given user ID.
func (j *JWT) Generate(userID uint64) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(userID, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.exp)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

// Validate parses and validates the token returning its claims.
func (j *JWT) Validate(tok string) (*jwt.RegisteredClaims, error) {
	parsed, err := jwt.ParseWithClaims(tok, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
