package registry

import "context"

// DBConfig specifies database connection parameters for registry operations.
type DBConfig struct {
	DSN         string
	Schema      string
	Driver      string
	TablePrefix string
}

// DefaultTablePrefix is applied when no prefix is provided.
const DefaultTablePrefix = "gcfm_"

// Prefix returns p when non-empty or DefaultTablePrefix otherwise.
func Prefix(p string) string {
	if p == "" {
		return DefaultTablePrefix
	}
	return p
}

// TableName returns the table name with the supplied prefix applied.
func TableName(prefix, name string) string {
	return Prefix(prefix) + name
}

// FieldMeta represents custom field metadata.
type FieldMeta struct {
	DBID            int64          `yaml:"dbId" json:"dbId"`
	TableName       string         `yaml:"table"`
	ColumnName      string         `yaml:"column"`
	DataType        string         `yaml:"type"`
	Placeholder     string         `yaml:"placeholder,omitempty"`
	Display         *DisplayMeta   `yaml:"display,omitempty"`
	Validator       string         `yaml:"validator,omitempty"`
	ValidatorParams map[string]any `yaml:"validatorParams,omitempty" json:"validatorParams,omitempty"`
	Nullable        bool           `yaml:"nullable,omitempty"`
	Unique          bool           `yaml:"unique,omitempty"`
	HasDefault      bool           `yaml:"hasDefault,omitempty" json:"hasDefault"`
	Default         *string        `yaml:"defaultValue,omitempty" json:"defaultValue,omitempty"`
}

// Scanner defines metadata scanning behaviour.
type Scanner interface {
	Scan(ctx context.Context, conf DBConfig) ([]FieldMeta, error)
}
