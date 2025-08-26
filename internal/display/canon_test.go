package display

import "testing"

func TestCanonicalizeWidgetID(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", "core://auto"},
		{"text", "plugin://text-input"},
		{"number-input", "plugin://number-input"},
		{"core://auto", "core://auto"},
		{"plugin://custom", "plugin://custom"},
		{"unknown", "plugin://unknown"},
	}
	for _, tt := range tests {
		if got := CanonicalizeWidgetID(tt.in); got != tt.out {
			t.Fatalf("CanonicalizeWidgetID(%q)=%q want %q", tt.in, got, tt.out)
		}
	}
}
