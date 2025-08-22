package util

// Deref returns the value of p if non-nil, otherwise an empty string.
func Deref(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// SanitizeLimit clamps limit to [1,200] and defaults to 50 when non-positive.
func SanitizeLimit(limit int) int {
	const (
		defaultLimit = 50
		maxLimit     = 200
	)
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
