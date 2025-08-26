package display

import "strings"

// CanonicalizeWidgetID converts shorthand or legacy widget identifiers into
// fully qualified IDs. It falls back to plugin-prefixed IDs for unknown names
// and returns "core://auto" when the input is empty.
func CanonicalizeWidgetID(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "core://auto"
	}
	if strings.Contains(s, "://") {
		return s
	}
	switch s {
	case "text", "textinput", "text-input":
		return "plugin://text-input"
	case "number", "number-input", "numeric":
		return "plugin://number-input"
	case "textarea":
		return "plugin://textarea"
	case "checkbox", "bool":
		return "plugin://checkbox"
	case "date", "date-input":
		return "plugin://date-input"
	case "time", "time-input":
		return "plugin://time-input"
	case "datetime", "datetime-input":
		return "plugin://datetime-input"
	case "json", "json-editor":
		return "plugin://json-editor"
	case "select":
		return "plugin://select"
	case "multiselect":
		return "plugin://multiselect"
	case "password", "password-input":
		return "plugin://password-input"
	case "email", "email-input":
		return "plugin://email-input"
	case "url", "url-input":
		return "plugin://url-input"
	case "uuid", "uuid-input":
		return "plugin://uuid-input"
	default:
		return "plugin://" + s
	}
}
