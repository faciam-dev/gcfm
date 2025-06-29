package schema

type CustomField struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Type   string `json:"type"`
}
