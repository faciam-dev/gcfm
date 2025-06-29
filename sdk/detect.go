package sdk

import (
	"fmt"
	"net/url"
)

func detectDriver(dsn string) (string, error) {
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
