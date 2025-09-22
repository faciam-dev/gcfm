package handler

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	capabilitydomain "github.com/faciam-dev/gcfm/internal/domain/capability"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	capabilityusecase "github.com/faciam-dev/gcfm/internal/usecase/capability"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

func TestListHandlesPlainDSN(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := &monitordb.Repo{DB: db, Dialect: ormdriver.MySQLDialect{}}

	rows := sqlmock.NewRows([]string{"id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at"}).
		AddRow(1, "t1", "db1", "mysql", "plain", []byte{}, time.Now())
	sqlStr, _, _ := query.New(db, "gcfm_monitored_databases", ormdriver.MySQLDialect{}).
		Select("id", "tenant_id", "name", "driver", "dsn", "dsn_enc", "created_at").
		Where("tenant_id", "t1").
		OrderBy("id", "asc").
		Build()
	mock.ExpectQuery(regexp.QuoteMeta(sqlStr)).WithArgs("t1").WillReturnRows(rows)

	h := &DatabaseHandler{Repo: repo}
	ctx := tenant.WithTenant(context.Background(), "t1")
	out, err := h.list(ctx, &struct{}{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.Body) != 1 {
		t.Fatalf("expected 1 database, got %d", len(out.Body))
	}
	if out.Body[0].DSN != "" || out.Body[0].DSNEnc != "" {
		t.Fatalf("unexpected dsn fields: %#v", out.Body[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("db expectations: %v", err)
	}
}

type stubCapService struct {
	caps capabilitydomain.Capabilities
	err  error
}

func (s stubCapService) Get(ctx context.Context, tenant string, dbID int64) (capabilitydomain.Capabilities, error) {
	return s.caps, s.err
}

func TestCapabilitiesSuccess(t *testing.T) {
	h := &DatabaseHandler{Capabilities: stubCapService{caps: capabilitydomain.Capabilities{Driver: "mongodb"}}}
	ctx := tenant.WithTenant(context.Background(), "default")
	out, err := h.capabilities(ctx, &idParam{ID: 1})
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if out.Body.Driver != "mongodb" {
		t.Fatalf("unexpected driver: %+v", out.Body)
	}
}

func TestCapabilitiesAdapterMissing(t *testing.T) {
	h := &DatabaseHandler{Capabilities: stubCapService{err: capabilityusecase.ErrAdapterNotFound}}
	ctx := tenant.WithTenant(context.Background(), "default")
	if _, err := h.capabilities(ctx, &idParam{ID: 2}); err == nil {
		t.Fatalf("expected error")
	}
}
