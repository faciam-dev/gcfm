package widgetpolicy

import (
	"strconv"
	"strings"
)

func NormalizeType(driver, in string, length *int) (out string, isBool bool) {
	t := strings.ToLower(strings.TrimSpace(in))
	if i := strings.IndexByte(t, '('); i > 0 {
		t = t[:i]
	}
	switch driver {
	case "mysql":
		if t == "tinyint" && length != nil && *length == 1 {
			return "boolean", true
		}
		if t == "timestamp" {
			return "datetime", false
		}
	case "postgres":
		if t == "timestamptz" {
			return "timestamptz", false
		}
	}
	switch t {
	case "char", "varchar", "text", "mediumtext", "longtext", "json", "jsonb", "date", "time", "datetime", "timestamptz", "bool", "boolean", "enum", "set", "int", "integer", "bigint", "smallint", "tinyint", "decimal", "numeric", "float", "double", "real":
		return t, t == "bool" || t == "boolean"
	default:
		return t, false
	}
}

func NormalizeValidator(in string) string {
	v := strings.ToLower(strings.TrimSpace(in))
	if v == "" {
		v = "none"
	}
	return v
}

func ParseTypeInfo(t string) (base string, length *int, enums []string) {
	s := strings.ToLower(strings.TrimSpace(t))
	base = s
	if i := strings.Index(s, "("); i >= 0 && strings.HasSuffix(s, ")") {
		inner := s[i+1 : len(s)-1]
		base = s[:i]
		if base == "enum" || base == "set" {
			parts := strings.Split(inner, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				p = strings.Trim(p, "'\"")
				enums = append(enums, p)
			}
		} else {
			seg := strings.Split(inner, ",")[0]
			if n, err := strconv.Atoi(strings.TrimSpace(seg)); err == nil {
				length = &n
			}
		}
	}
	return
}
