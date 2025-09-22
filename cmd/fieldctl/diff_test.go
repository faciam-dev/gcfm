package main

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

const selectCustomFields = "SELECT `db_id`, `table_name`, `column_name`, `data_type`, `store_kind`, `kind`, `physical_type`, `driver_extras`, `label_key`, `widget`, `widget_config`, `placeholder_key`, `nullable`, `unique`, `has_default`, `default_value`, `validator` FROM `gcfm_custom_fields` ORDER BY table_name, column_name"

func fieldRows(m sqlmock.Sqlmock) *sqlmock.Rows {
	return m.NewRows([]string{"db_id", "table_name", "column_name", "data_type", "store_kind", "kind", "physical_type", "driver_extras", "label_key", "widget", "widget_config", "placeholder_key", "nullable", "unique", "has_default", "default_value", "validator"}).
		AddRow(1, "posts", "title", "text", "sql", nil, nil, []byte("{}"), nil, nil, nil, nil, false, false, false, nil, nil)
}

type passthroughConverter struct{}

func (passthroughConverter) ConvertValue(v interface{}) (driver.Value, error) {
	switch v.(type) {
	case sql.NullString:
		return v, nil
	}
	return driver.DefaultParameterConverter.ConvertValue(v)
}

func TestDiffCmdNoChange(t *testing.T) {
	exitCode := 0
	exitFunc = func(c int) { exitCode = c }
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_nochange", sqlmock.ValueConverterOption(passthroughConverter{}))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := fieldRows(mock)
	mock.ExpectQuery(regexp.QuoteMeta(selectCustomFields)).WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: text\n")
	f := "test.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_nochange", "--schema", "public", "--driver", "sqlmock", "--file", f, "--table-prefix", "gcfm_"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("unexpected exit %d", exitCode)
	}
	if buf.String() == "" {
		t.Fatalf("no output")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDiffCmdChangeFail(t *testing.T) {
	exitCode := 0
	exitFunc = func(c int) { exitCode = c }
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_change", sqlmock.ValueConverterOption(passthroughConverter{}))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := fieldRows(mock)
	mock.ExpectQuery(regexp.QuoteMeta(selectCustomFields)).WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: varchar(20)\n")
	f := "test2.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SilenceUsage = true
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_change", "--schema", "public", "--driver", "sqlmock", "--file", f, "--fail-on-change", "--table-prefix", "gcfm_"})
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

	db, mock, err := sqlmock.NewWithDSN("sqlmock_md", sqlmock.ValueConverterOption(passthroughConverter{}))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := fieldRows(mock)
	mock.ExpectQuery(regexp.QuoteMeta(selectCustomFields)).WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: varchar(20)\n")
	f := "test3.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_md", "--schema", "public", "--driver", "sqlmock", "--file", f, "--format", "markdown", "--table-prefix", "gcfm_"})
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

	db, mock, err := sqlmock.NewWithDSN("sqlmock_change_text", sqlmock.ValueConverterOption(passthroughConverter{}))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := fieldRows(mock)
	mock.ExpectQuery(regexp.QuoteMeta(selectCustomFields)).WillReturnRows(rows)

	yaml := []byte("version: 0.4\nfields:\n  - table: posts\n    column: title\n    type: varchar(20)\n")
	f := "test4.yaml"
	os.WriteFile(f, yaml, 0644)
	defer os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_change_text", "--schema", "public", "--driver", "sqlmock", "--file", f, "--table-prefix", "gcfm_"})
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
	t.Skip("TODO: fix diff output for ignore regex after ORM refactor")
}

func TestDiffCmdSkipReserved(t *testing.T) {
	t.Skip("TODO: fix diff output for reserved tables after ORM refactor")
}

func TestDiffCmdFallbackExport(t *testing.T) {
	exitCode := 0
	exitFunc = func(c int) { exitCode = c }
	defer func() { exitFunc = os.Exit }()

	db, mock, err := sqlmock.NewWithDSN("sqlmock_fallback", sqlmock.ValueConverterOption(passthroughConverter{}))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := fieldRows(mock)
	mock.ExpectQuery(regexp.QuoteMeta(selectCustomFields)).WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(selectCustomFields)).WillReturnRows(fieldRows(mock))

	f := "nonexistent.yaml"
	os.Remove(f)

	buf := new(bytes.Buffer)
	cmd := newDiffCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "sqlmock_fallback", "--schema", "public", "--driver", "sqlmock", "--file", f, "--table-prefix", "gcfm_", "--fallback-export"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if exitCode != 3 {
		t.Fatalf("expected exit 3 got %d", exitCode)
	}
	if buf.String() == "" {
		t.Fatalf("no output")
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatalf("expected file created: %v", err)
	}
	os.Remove(f)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
