package registrycmd

import (
	"log"
	"net/url"
)

// detectDriver returns the driver name based on the DSN scheme.
func detectDriver(dsn string) string {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		log.Printf("Error parsing DSN: %v", err)
		return "unknown"
	}
	switch parsedURL.Scheme {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return "unknown"
	}
}
