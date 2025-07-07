package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDiffCmdNoChange(t *testing.T) {
	exitCode := 0
	exitFunc = func(c int) { exitCode = c }
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_nochange")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).AddRow("posts", "title", "text", nil, "text", nil, false, false, false, nil, nil)
	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	f := "test.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_nochange", "--schema", "public", "--driver", "sqlmock", "--file", f})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("unexpected exit %d", exitCode)
	}
	if buf.String() != "✅ No schema drift detected.\n" {
		t.Fatalf("unexpected output: %s", buf.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDiffCmdChangeFail(t *testing.T) {
	exitCode := 0
	exitFunc = func(c int) { exitCode = c }
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_change")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).AddRow("posts", "title", "text", nil, "text", nil, false, false, false, nil, nil)
	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: varchar(20)\n")
	f := "test2.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_change", "--schema", "public", "--driver", "sqlmock", "--file", f, "--fail-on-change"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if exitCode != 2 {
		t.Fatalf("expected exit 2 got %d", exitCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDiffCmdMarkdown(t *testing.T) {
	exitFunc = func(int) {}
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_md")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).AddRow("posts", "title", "text", nil, "text", nil, false, false, false, nil, nil)
	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: varchar(20)\n")
	f := "test3.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_md", "--schema", "public", "--driver", "sqlmock", "--file", f, "--format", "markdown"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if len(out) == 0 || out[:7] != "```diff" {
		t.Fatalf("markdown output not diff: %s", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDiffCmdChangeTextNoFail(t *testing.T) {
	exitCode := 0
	exitFunc = func(c int) { exitCode = c }
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_change_text")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).AddRow("posts", "title", "text", nil, "text", nil, false, false, false, nil, nil)
	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: varchar(20)\n")
	f := "test4.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_change_text", "--schema", "public", "--driver", "sqlmock", "--file", f})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("unexpected exit %d", exitCode)
	}
	out := buf.String()
	if !strings.Contains(out, "± posts.title type: text → varchar(20)") {
		t.Fatalf("unexpected output: %s", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDiffCmdIgnoreRegex(t *testing.T) {
	exitFunc = func(int) {}
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_ignore")
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "label_key", "widget", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).
		AddRow("posts", "title", "text", nil, "text", nil, false, false, false, nil, nil).
		AddRow("gcfm_meta", "id", "int", nil, "text", nil, false, false, false, nil, nil)
	mock.ExpectQuery("^SELECT table_name, column_name, data_type, label_key, widget, placeholder_key, nullable, `unique`, has_default, default_value, validator FROM gcfm_custom_fields ORDER BY table_name, column_name$").WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	f := "test_ignore.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_ignore", "--schema", "public", "--driver", "sqlmock", "--file", f, "--ignore-regex", "^gcfm_"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if out != "✅ No schema drift detected.\n" {
		t.Fatalf("unexpected output: %s", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
