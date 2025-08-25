package handler

import "testing"

func TestCanonicalizeWidgetID(t *testing.T) {
	tests := []struct {
		in      string
		colType string
		want    string
		auto    bool
	}{
		{"core://default", "text", "core://auto", true},
		{"core://auto", "text", "core://auto", true},
		{"", "text", "core://auto", true},
		{"core://text-input", "text", "plugin://text-input", false},
		{"core://date-input", "date", "plugin://date-input", false},
		{"plugin://text-input", "text", "plugin://text-input", false},
		{"text", "text", "text", false},
	}
	for _, tt := range tests {
		got, _, isAuto := canonicalizeWidgetID(tt.in, tt.colType)
		if got != tt.want || isAuto != tt.auto {
			t.Fatalf("canonicalizeWidgetID(%q)=%q auto=%v want %q auto=%v", tt.in, got, isAuto, tt.want, tt.auto)
		}
	}
}
