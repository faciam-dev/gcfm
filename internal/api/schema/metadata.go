package schema

// TableMeta represents a database table.
type TableMeta struct {
	Table   string `json:"table"`
	Comment string `json:"comment,omitempty"`
}
