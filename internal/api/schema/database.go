package schema

import "time"

// Database represents a monitored database record.
type Database struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Driver    string    `json:"driver"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateDatabase is the input for creating a monitored database.
type CreateDatabase struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}
