package util

import (
	"fmt"
	"net/url"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
)

// UnsupportedDialect is returned when a driver has no corresponding goquent dialect.
type UnsupportedDialect struct{ Driver string }

func (UnsupportedDialect) Placeholder(int) string { return "?" }

func (UnsupportedDialect) QuoteIdent(ident string) string { return ident }

// DetectDriver returns the driver name based on the DSN scheme.
// Supported schemes: mysql, postgres/postgresql and mongodb/mongodb+srv.
func DetectDriver(dsn string) (string, error) {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	switch parsedURL.Scheme {
	case "postgres", "postgresql":
		return "postgres", nil
	case "mongodb", "mongodb+srv":
		return "mongo", nil
	case "mysql":
		return "mysql", nil
	default:
		return "", fmt.Errorf("unknown scheme: %s", parsedURL.Scheme)
	}
}

// DialectFromDriver returns the goquent dialect corresponding to a driver.
func DialectFromDriver(d string) ormdriver.Dialect {
	switch d {
	case "postgres":
		return ormdriver.PostgresDialect{}
	case "mysql":
		return ormdriver.MySQLDialect{}
	default:
		return UnsupportedDialect{Driver: d}
	}
}
