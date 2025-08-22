package widgetsrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func TestPGRepoList(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewPGRepo(db)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at"}).
		AddRow("a", "A", "1", "widget", pq.StringArray{"system"}, true, sql.NullString{}, pq.StringArray{}, sql.NullString{}, []byte(`{}`), "system", pq.StringArray{}, now)
	mock.ExpectQuery("WITH base").WithArgs(sqlmock.AnyArg(), "", "", 10, 0).WillReturnRows(rows)
	mock.ExpectQuery("WITH base").WithArgs(sqlmock.AnyArg(), "", "").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	items, total, err := repo.List(context.Background(), Filter{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || total != 1 {
		t.Fatalf("unexpected result: %+v total=%d", items, total)
	}
}

func TestPGRepoGetETagAndLastMod(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewPGRepo(db)
	now := time.Now()
	mock.ExpectQuery("WITH base").WithArgs(sqlmock.AnyArg(), "", "").WillReturnRows(sqlmock.NewRows([]string{"etag", "last_mod"}).AddRow("abc", now))
	etag, last, err := repo.GetETagAndLastMod(context.Background(), Filter{})
	if err != nil {
		t.Fatalf("etag: %v", err)
	}
	if etag == "" || last.IsZero() {
		t.Fatalf("unexpected etag or last: %s %v", etag, last)
	}
}

func TestPGRepoUpsertAndRemove(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewPGRepo(db)
	r := Row{ID: "a", Name: "A", Version: "1", Type: "widget", Scopes: []string{"system"}, Enabled: true, Meta: map[string]any{"k": "v"}, TenantScope: "system"}
	mock.ExpectExec("INSERT INTO gcfm_widgets").WithArgs("a", "A", "1", "widget", sqlmock.AnyArg(), true, nil, sqlmock.AnyArg(), nil, sqlmock.AnyArg(), "system", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	if err := repo.Upsert(context.Background(), r); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	mock.ExpectExec("DELETE FROM gcfm_widgets").WithArgs("a").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.Remove(context.Background(), "a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestPGRepoGetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewPGRepo(db)
	now := time.Now()
	row := sqlmock.NewRows([]string{"id", "name", "version", "type", "scopes", "enabled", "description", "capabilities", "homepage", "meta", "tenant_scope", "tenants", "updated_at"}).
		AddRow("a", "A", "1", "widget", pq.StringArray{"system"}, true, sql.NullString{String: "desc", Valid: true}, pq.StringArray{}, sql.NullString{}, []byte(`{"k":"v"}`), "system", pq.StringArray{}, now)
	mock.ExpectQuery("SELECT id, name, version").WithArgs("a").WillReturnRows(row)
	got, err := repo.GetByID(context.Background(), "a")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != "a" || got.Description == nil || *got.Description != "desc" || got.Meta["k"] != "v" {
		b, _ := json.Marshal(got)
		t.Fatalf("unexpected row: %s", string(b))
	}
}
