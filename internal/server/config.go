package server

// DBConfig holds database configuration for the API server.
type DBConfig struct {
	Driver      string
	DSN         string
	TablePrefix string
}
