package snapshot_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/snapshot"
	"gopkg.in/yaml.v3"
)

func TestExportLocal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	rows := sqlmock.NewRows([]string{"id", "table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "default", "validator"}).AddRow(1, "posts", "title", "text", "", "", "", false, false, "", "")
	mock.ExpectQuery("^SELECT id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, `default`, validator FROM custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)
	dir := t.TempDir()
	if err := snapshot.Export(context.Background(), db, "", "mysql", snapshot.LocalDir{Path: dir}); err != nil {
		t.Fatalf("export: %v", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v %d", err, len(files))
	}
	b, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var res struct {
		Version string               `yaml:"version"`
		Fields  []registry.FieldMeta `yaml:"fields"`
	}
	if err := yaml.Unmarshal(b, &res); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	if len(res.Fields) != 1 || res.Fields[0].TableName != "posts" {
		t.Fatalf("unexpected meta: %#v", res.Fields)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestExportLocalPostgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	rows := sqlmock.NewRows([]string{"id", "table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "default", "validator"}).AddRow(1, "posts", "title", "text", "", "", "", false, false, "", "")
	mock.ExpectQuery("^SELECT id, table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, \"unique\", \"default\", validator FROM custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)
	dir := t.TempDir()
	if err := snapshot.Export(context.Background(), db, "", "postgres", snapshot.LocalDir{Path: dir}); err != nil {
		t.Fatalf("export: %v", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v %d", err, len(files))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
