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

// SkipInfo describes why a field was skipped during scanning.
type SkipInfo struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Reason string `json:"reason"`
}

// ScanResult summarizes a scan operation.
type ScanResult struct {
	Total      int        `json:"total"`
	Inserted   int        `json:"inserted"`
	Updated    int        `json:"updated"`
	Skipped    int        `json:"skipped"`
	SkipDetail []SkipInfo `json:"skippedList,omitempty"`
}
