package unit_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
)

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
func int64Ptr(i int64) *int64 { return &i }

func TestCustomFieldValidation(t *testing.T) {
	reg := huma.NewMapRegistry("#/components/schemas", huma.DefaultSchemaNamer)
	sch := huma.SchemaFromType(reg, reflect.TypeOf(schema.CustomField{}))
	pb := huma.NewPathBuffer([]byte(""), 0)
	cf := schema.CustomField{
		DBID:         int64Ptr(1),
		Table:        "posts",
		Column:       "id",
		Type:         "uuid",
		Display:      schema.DisplaySettings{Widget: "text"},
		Nullable:     boolPtr(true),
		Unique:       boolPtr(false),
		HasDefault:   true,
		DefaultValue: strPtr("foo"),
		Validator:    "uuid",
	}
	b, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	res := &huma.ValidateResult{}
	huma.Validate(reg, sch, pb, huma.ModeWriteToServer, m, res)
	if len(res.Errors) > 0 {
		t.Fatalf("validation errors: %v", res.Errors)
	}
}
