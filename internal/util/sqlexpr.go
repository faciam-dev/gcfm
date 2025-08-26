package util

import "strings"

var sqlExpressions = map[string]struct{}{
	"CURRENT_TIMESTAMP": {},
	"NOW()":             {},
	"UTC_TIMESTAMP()":   {},
	"CURRENT_DATE":      {},
	"CURRENT_TIME":      {},
	"CURDATE()":         {},
	"CURTIME()":         {},
}

// IsSQLExpression reports whether raw looks like a recognized SQL temporal expression.
func IsSQLExpression(raw string) bool {
	token := strings.ToUpper(strings.TrimSpace(raw))
	if _, ok := sqlExpressions[token]; ok {
		return true
	}
	return strings.HasPrefix(token, "CURRENT_TIMESTAMP(")
}
