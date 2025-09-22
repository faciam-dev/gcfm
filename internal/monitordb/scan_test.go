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

func TestSchemaFromDSN_Mongo(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{name: "with database", dsn: "mongodb://user:pass@localhost:27017/sample", want: "sample"},
		{name: "trim leading slash", dsn: "mongodb://localhost:27017//sample", want: "sample"},
		{name: "srv", dsn: "mongodb+srv://example.com/mydb?ssl=true", want: "mydb"},
		{name: "auth source", dsn: "mongodb://user:pass@localhost:27017/?authSource=admin", want: "admin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := schemaFromDSN("mongo", tt.dsn); got != tt.want {
				t.Fatalf("schemaFromDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}
