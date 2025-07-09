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

// Claims represents the JWT claims used by this service. The tenant ID is stored
// separately from the standard registered claims so that it is explicit.
type Claims struct {
	jwt.RegisteredClaims
	TenantID string `json:"tid,omitempty"`
}

// GetTenantID returns the tenant ID claim.
func (c *Claims) GetTenantID() string { return c.TenantID }

// NewJWT returns a new JWT handler.
func NewJWT(secret string, exp time.Duration) *JWT {
	return &JWT{secret: []byte(secret), exp: exp}
}

// Generate creates a signed token for the given user ID.
func (j *JWT) Generate(userID uint64) (string, error) {
	return j.GenerateWithTenant(userID, "")
}

// GenerateWithTenant creates a signed token for the given user ID and tenant ID.
// If tenantID is empty, then the claim will be omitted.
func (j *JWT) GenerateWithTenant(userID uint64, tenantID string) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.exp)),
		},
	}
	if tenantID != "" {
		claims.TenantID = tenantID
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

// Validate parses and validates the token returning its claims.
func (j *JWT) Validate(tok string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tok, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
