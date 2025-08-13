package metadata

import (
	"regexp"
	"strings"
)

// TableInfo represents table metadata including schema information.
type TableInfo struct {
	Schema    string
	Name      string
	Qualified string
}

// excludeRule holds driver-specific exclusion rules.
type excludeRule struct {
	Schemas []string
	Exact   map[string]bool
	Prefix  []string
	Regex   []*regexp.Regexp
}

const defaultPrefix = "gcfm_"

var rules = map[string]excludeRule{
	"postgres": {
		Schemas: []string{"pg_catalog", "information_schema", "pg_toast"},
		Exact: map[string]bool{
			"schema_migrations":     true,
			"goose_db_version":      true,
			"flyway_schema_history": true,
		},
		Prefix: []string{
			defaultPrefix,
			"pg_temp_",
		},
		Regex: []*regexp.Regexp{
			regexp.MustCompile(`^pg_.*`),
		},
	},
	"mysql": {
		Exact: map[string]bool{
			"schema_migrations":     true,
			"goose_db_version":      true,
			"flyway_schema_history": true,
		},
		Prefix: []string{
			defaultPrefix,
		},
	},
}

// SetTablePrefix updates the exclusion rules with the provided table prefix.
// An empty prefix is ignored to avoid filtering out every table.
func SetTablePrefix(p string) {
        lp := strings.ToLower(p)
        if lp == "" {
                return
        }
        if r, ok := rules["postgres"]; ok {
                if len(r.Prefix) > 0 {
                        r.Prefix[0] = lp
                        rules["postgres"] = r
                }
        }
        if r, ok := rules["mysql"]; ok {
                if len(r.Prefix) > 0 {
                        r.Prefix[0] = lp
                        rules["mysql"] = r
                }
        }
}

func shouldExclude(driver string, t TableInfo) bool {
	r, ok := rules[strings.ToLower(driver)]
	if !ok {
		r = excludeRule{}
	}
	for _, s := range r.Schemas {
		if t.Schema == s {
			return true
		}
	}
	schemaLower := strings.ToLower(t.Schema)
	for _, p := range r.Prefix {
		if strings.HasPrefix(schemaLower, strings.ToLower(p)) {
			return true
		}
	}
	name := strings.ToLower(t.Name)
	if r.Exact[name] {
		return true
	}
	for _, p := range r.Prefix {
		if strings.HasPrefix(name, strings.ToLower(p)) {
			return true
		}
	}
	for _, rx := range r.Regex {
		if rx != nil && rx.MatchString(name) {
			return true
		}
	}
	return false
}

// FilterTables filters tables based on driver-specific rules.
func FilterTables(driver string, in []TableInfo) (out []TableInfo) {
	out = make([]TableInfo, 0, len(in))
	for _, t := range in {
		if !shouldExclude(driver, t) {
			out = append(out, t)
		}
	}
	return
}
