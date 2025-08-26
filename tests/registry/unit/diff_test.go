package unit_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/faciam-dev/gcfm/pkg/registry"
)

func TestDiff(t *testing.T) {
	original := []registry.FieldMeta{{TableName: "posts", ColumnName: "name", DataType: "varchar"}}
	modified := []registry.FieldMeta{{TableName: "posts", ColumnName: "name", DataType: "text"}, {TableName: "posts", ColumnName: "age", DataType: "int"}}

	changes := registry.Diff(original, modified)
	var got = map[registry.ChangeType]int{}
	for _, c := range changes {
		got[c.Type]++
	}

	want := map[registry.ChangeType]int{registry.ChangeUpdated: 1, registry.ChangeAdded: 1}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected diff counts (-want +got):\n%s", diff)
	}
}

func TestDiffDeleted(t *testing.T) {
	original := []registry.FieldMeta{{TableName: "posts", ColumnName: "name", DataType: "varchar"}}
	modified := []registry.FieldMeta{}

	changes := registry.Diff(original, modified)
	if len(changes) != 1 || changes[0].Type != registry.ChangeDeleted {
		t.Fatalf("expected deleted change")
	}
}
