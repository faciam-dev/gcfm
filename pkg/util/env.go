package util

import "os"

// GetEnv returns the value of the environment variable named by key or def if empty.
func GetEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
