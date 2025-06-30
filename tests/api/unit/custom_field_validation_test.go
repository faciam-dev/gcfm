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

func TestCustomFieldValidation(t *testing.T) {
	reg := huma.NewMapRegistry("#/components/schemas", huma.DefaultSchemaNamer)
	sch := huma.SchemaFromType(reg, reflect.TypeOf(schema.CustomField{}))
	pb := huma.NewPathBuffer([]byte(""), 0)
	cf := schema.CustomField{
		Table:     "posts",
		Column:    "id",
		Type:      "uuid",
		Nullable:  boolPtr(true),
		Unique:    boolPtr(false),
		Default:   strPtr("foo"),
		Validator: "uuid",
	}
	b, _ := json.Marshal(cf)
	var m map[string]any
	json.Unmarshal(b, &m)
	res := &huma.ValidateResult{}
	huma.Validate(reg, sch, pb, huma.ModeWriteToServer, m, res)
	if len(res.Errors) > 0 {
		t.Fatalf("validation errors: %v", res.Errors)
	}
}
