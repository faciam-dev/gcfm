package schema

import "time"

// Target represents a monitored database target configuration.
type Target struct {
	Key           string    `json:"key"`
	Driver        string    `json:"driver"`
	DSN           string    `json:"dsn"`
	Schema        string    `json:"schema,omitempty"`
	Labels        []string  `json:"labels"`
	MaxOpenConns  int       `json:"maxOpenConns,omitempty" validate:"omitempty,min=0"`
	MaxIdleConns  int       `json:"maxIdleConns,omitempty" validate:"omitempty,min=0"`
	ConnMaxIdleMs int       `json:"connMaxIdleMs,omitempty" validate:"omitempty,min=0"`
	ConnMaxLifeMs int       `json:"connMaxLifeMs,omitempty" validate:"omitempty,min=0"`
	IsDefault     bool      `json:"isDefault,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt,omitempty"`
}

// TargetInput is used for create or full upsert operations.
type TargetInput struct {
	Target
}

// TargetPatch defines fields that can be updated via PATCH.
type TargetPatch struct {
	Driver        *string  `json:"driver,omitempty"`
	DSN           *string  `json:"dsn,omitempty"`
	Schema        *string  `json:"schema,omitempty"`
	Labels        []string `json:"labels,omitempty"`
	MaxOpenConns  *int     `json:"maxOpenConns,omitempty" validate:"omitempty,min=0"`
	MaxIdleConns  *int     `json:"maxIdleConns,omitempty" validate:"omitempty,min=0"`
	ConnMaxIdleMs *int     `json:"connMaxIdleMs,omitempty" validate:"omitempty,min=0"`
	ConnMaxLifeMs *int     `json:"connMaxLifeMs,omitempty" validate:"omitempty,min=0"`
}

// TargetsList represents a list of targets with pagination information.
type TargetsList struct {
	Items      []Target `json:"items"`
	NextCursor string   `json:"nextCursor,omitempty"`
}
