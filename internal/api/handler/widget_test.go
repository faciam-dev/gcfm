package handler

import "testing"

func TestCanonicalizeWidgetID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"core://default", "plugin://text-input"},
		{"core://text-input", "plugin://text-input"},
		{"core://date-input", "plugin://date-input"},
		{"plugin://text-input", "plugin://text-input"},
		{"text", "text"},
	}
	for _, tt := range tests {
		got, _ := canonicalizeWidgetID(tt.in)
		if got != tt.want {
			t.Fatalf("canonicalizeWidgetID(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}
