package migrator

import "testing"

func TestSplitSQLDollarQuote(t *testing.T) {
	src := "CREATE OR REPLACE FUNCTION notify_widgets_changed() RETURNS trigger LANGUAGE plpgsql AS $$\nBEGIN\n  PERFORM pg_notify('widgets_changed', COALESCE(NEW.id, OLD.id));\n  RETURN NEW;\nEND;\n$$;"
	stmts := splitSQL(src)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %#v", len(stmts), stmts)
	}
}
