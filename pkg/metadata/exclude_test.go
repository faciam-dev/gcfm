package metadata

import "testing"

func TestFilter_Postgres(t *testing.T) {
	in := []TableInfo{
		{Schema: "pg_catalog", Name: "pg_proc"},
		{Schema: "pg_temp_3", Name: "temp_table"},
		{Schema: "public", Name: "schema_migrations"},
		{Schema: "public", Name: "gcfm_audit_logs"},
		{Schema: "public", Name: "users"},
		{Schema: "public", Name: "GCFM_custom_fields"},
	}
	out := FilterTables("postgres", in)
	got := names(out)
	want := []string{"users"}
	compare(t, got, want)
}

func TestFilter_MySQL(t *testing.T) {
	in := []TableInfo{
		{Name: "schema_migrations"},
		{Name: "gcfm_registry_schema_version"},
		{Name: "users"},
	}
	out := FilterTables("mysql", in)
	got := names(out)
	want := []string{"users"}
	compare(t, got, want)
}

func TestSetTablePrefix_Empty(t *testing.T) {
	// backup original rules
	orig := rules["postgres"]
	SetTablePrefix("")
	in := []TableInfo{{Schema: "public", Name: "users"}}
	out := FilterTables("postgres", in)
	got := names(out)
	want := []string{"users"}
	compare(t, got, want)
	// restore original rules
	rules["postgres"] = orig
}

func names(ts []TableInfo) []string {
	r := make([]string, 0, len(ts))
	for _, t := range ts {
		r = append(r, t.Name)
	}
	return r
}

func compare(t *testing.T, got, want []string) {
	if len(got) != len(want) {
		t.Fatalf("len got=%d want=%d (%v)", len(got), len(want), got)
	}
	m := map[string]bool{}
	for _, w := range want {
		m[w] = true
	}
	for _, g := range got {
		if !m[g] {
			t.Fatalf("unexpected: %v, want %v", g, want)
		}
	}
}
