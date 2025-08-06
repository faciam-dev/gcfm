package schema

// Field represents a registry field used in snapshots.
type Field struct {
	Table  string `yaml:"table"`
	Column string `yaml:"column"`
	Type   string `yaml:"type"`
}
