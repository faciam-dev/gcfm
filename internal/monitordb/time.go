package monitordb

import (
	"errors"
	"time"
)

// parseSQLTime converts various database time representations into time.Time.
func parseSQLTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case []byte:
		return parseSQLTimeString(string(t))
	case string:
		return parseSQLTimeString(t)
	default:
		return time.Time{}, errors.New("unsupported time type")
	}
}

func parseSQLTimeString(s string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", time.RFC3339}
	for _, l := range layouts {
		if ts, err := time.Parse(l, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, errors.New("cannot parse time: " + s)
}
