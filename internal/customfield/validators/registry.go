package validators

import "strings"

// Validator describes a field validator and its applicability.
type Validator struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	AppliesTo   []string       `json:"applies_to"`
	TableMatch  []string       `json:"table_match"`
	Params      map[string]any `json:"params"`
	Schema      map[string]any `json:"schema"`
}

func builtin() []Validator {
	return []Validator{
		{ID: "none", Name: "none", Description: "No constraints", AppliesTo: []string{"*"}},
		{ID: "email", Name: "email", Description: "Email format", AppliesTo: []string{"varchar", "text"}},
		{ID: "number", Name: "number", Description: "Numeric value", AppliesTo: []string{"int", "bigint", "decimal", "float", "double"}},
		{ID: "uuid", Name: "uuid", Description: "UUID format", AppliesTo: []string{"uuid", "varchar"}},
		{ID: "regex", Name: "regex", Description: "Regular expression", AppliesTo: []string{"varchar", "text"},
			Params: map[string]any{"pattern": "^.*$"},
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"pattern": map[string]any{"type": "string", "title": "Pattern (PCRE)"}},
				"required":   []string{"pattern"},
			},
		},
	}
}

func matchesTable(v Validator, table string) bool {
	if len(v.TableMatch) == 0 || table == "" {
		return true
	}
	t := strings.ToLower(table)
	for _, m := range v.TableMatch {
		m = strings.ToLower(m)
		if strings.HasSuffix(m, "*") {
			if strings.HasPrefix(t, strings.TrimSuffix(m, "*")) {
				return true
			}
		} else if t == m {
			return true
		}
	}
	return false
}

func supportsType(v Validator, typ string) bool {
	typ = strings.ToLower(typ)
	for _, a := range v.AppliesTo {
		if a == "*" || strings.ToLower(a) == typ {
			return true
		}
	}
	return false
}

// Filter returns validators applicable to the provided selection.
func Filter(db, table, typ string) []Validator {
	res := make([]Validator, 0, 8)
	for _, v := range builtin() {
		if supportsType(v, typ) && matchesTable(v, table) {
			res = append(res, v)
		}
	}
	return res
}
