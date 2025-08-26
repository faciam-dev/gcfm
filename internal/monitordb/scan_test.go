package monitordb

import (
	"testing"

	"github.com/faciam-dev/gcfm/pkg/registry"
)

func TestMergeValidators(t *testing.T) {
	metas := []registry.FieldMeta{
		{TableName: "posts", ColumnName: "email"},
		{TableName: "posts", ColumnName: "title", Validator: "uuid"},
	}
	existing := []registry.FieldMeta{
		{TableName: "posts", ColumnName: "email", Validator: "email"},
		{TableName: "posts", ColumnName: "title", Validator: "email"},
	}
	mergeValidators(metas, existing)
	if metas[0].Validator != "email" {
		t.Fatalf("expected validator preserved, got %q", metas[0].Validator)
	}
	if metas[1].Validator != "uuid" {
		t.Fatalf("unexpected overwrite: %q", metas[1].Validator)
	}
}
