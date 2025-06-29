package sdk

import "net/url"

func detectDriver(dsn string) string {
	parsedURL, err := url.Parse(dsn)
	if err != nil {
		return "unknown"
	}
	switch parsedURL.Scheme {
	case "postgres", "postgresql":
		return "postgres"
	case "mongodb", "mongodb+srv":
		return "mongo"
	case "mysql":
		return "mysql"
	default:
		return "unknown"
	}
}
