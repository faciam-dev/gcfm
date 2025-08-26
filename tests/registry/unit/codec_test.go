package unit_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/faciam-dev/gcfm/pkg/registry"
	"github.com/faciam-dev/gcfm/pkg/registry/codec"
)

func TestCodecRoundTrip(t *testing.T) {
	in := []registry.FieldMeta{
		{
			TableName:  "posts",
			ColumnName: "author_email",
			DataType:   "varchar(255)",
			Display:    &registry.DisplayMeta{PlaceholderKey: "foo", Widget: "text"},
			Validator:  "email",
		},
		{
			TableName:  "posts",
			ColumnName: "rating",
			DataType:   "int",
			Display:    &registry.DisplayMeta{Widget: "number"},
		},
	}
	b, err := codec.EncodeYAML(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := codec.DecodeYAML(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if diff := cmp.Diff(in, out); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestDecodeV1(t *testing.T) {
	yamlData := []byte(`version: 0.1
fields:
  - table: posts
    column: author_email
    type: varchar(255)
    placeholder: foo@example.com
    validator: email
`)
	metas, err := codec.DecodeYAML(yamlData)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("unexpected length: got %d, want 1", len(metas))
	}
	m := metas[0]
	want := registry.DisplayMeta{PlaceholderKey: "foo@example.com", Widget: "text"}
	if diff := cmp.Diff(&want, m.Display); diff != "" {
		t.Fatalf("display mismatch (-want +got):\n%s", diff)
	}
}
