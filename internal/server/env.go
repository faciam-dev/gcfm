package server

import (
	"os"
	"strings"

	"github.com/faciam-dev/gcfm/internal/logger"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
)

// allowedOrigins returns the list of origins allowed for CORS.
func allowedOrigins() []string {
	allowed := pkgutil.GetEnv("ALLOWED_ORIGINS", "http://localhost:5173")
	origins := strings.Split(allowed, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return origins
}

// mustJWTSecret fetches JWT_SECRET or exits if unset.
func mustJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		logger.L.Error("JWT_SECRET environment variable is not set. Application cannot start.")
		os.Exit(1)
	}
	return secret
}
