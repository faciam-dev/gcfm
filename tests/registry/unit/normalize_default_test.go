package unit_test

import (
	"testing"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

func TestNormalizeDefaultForType(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		colType string
		in      registry.UnifiedDefault
		exp     registry.UnifiedDefault
		action  string
	}{
		{
			name:    "map expression to date",
			driver:  "mysql",
			colType: "date",
			in:      registry.UnifiedDefault{Mode: "expression", Raw: "CURRENT_TIMESTAMP", OnUpdate: true},
			exp:     registry.UnifiedDefault{Mode: "expression", Raw: "CURRENT_DATE", OnUpdate: false},
			action:  "mapped",
		},
		{
			name:    "clear expression for varchar",
			driver:  "mysql",
			colType: "varchar",
			in:      registry.UnifiedDefault{Mode: "expression", Raw: "NOW()"},
			exp:     registry.UnifiedDefault{Mode: "none", Raw: "", OnUpdate: false},
			action:  "cleared",
		},
		{
			name:    "trim literal for time",
			driver:  "mysql",
			colType: "time",
			in:      registry.UnifiedDefault{Mode: "literal", Raw: "2024-01-02 03:04:05", OnUpdate: true},
			exp:     registry.UnifiedDefault{Mode: "literal", Raw: "03:04:05", OnUpdate: false},
			action:  "mapped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := registry.NormalizeDefaultForType(tt.driver, tt.colType, tt.in)
			if res.Action != tt.action {
				t.Fatalf("action=%s want %s", res.Action, tt.action)
			}
			if res.Default != tt.exp {
				t.Fatalf("default=%+v want %+v", res.Default, tt.exp)
			}
		})
	}
}
