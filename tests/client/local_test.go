package client_test

import (
	"context"
	"testing"
	"time"

	sdk "github.com/faciam-dev/gcfm/sdk"
	client "github.com/faciam-dev/gcfm/sdk/client"
)

type stubService struct {
	listed  bool
	created bool
	updated bool
	deleted bool
}

func (s *stubService) ListCustomFields(ctx context.Context, dbID int64, table string) ([]sdk.FieldMeta, error) {
	s.listed = true
	return []sdk.FieldMeta{{TableName: "t", ColumnName: "c", DataType: "text"}}, nil
}
func (s *stubService) CreateCustomField(ctx context.Context, fm sdk.FieldMeta) error {
	s.created = true
	return nil
}
func (s *stubService) UpdateCustomField(ctx context.Context, fm sdk.FieldMeta) error {
	s.updated = true
	return nil
}
func (s *stubService) DeleteCustomField(ctx context.Context, table, column string) error {
	s.deleted = true
	return nil
}
func (s *stubService) Scan(context.Context, sdk.DBConfig) ([]sdk.FieldMeta, error) { return nil, nil }
func (s *stubService) Export(context.Context, sdk.DBConfig) ([]byte, error)        { return nil, nil }
func (s *stubService) Apply(context.Context, sdk.DBConfig, []byte, sdk.ApplyOptions) (sdk.DiffReport, error) {
	return sdk.DiffReport{}, nil
}
func (s *stubService) MigrateRegistry(context.Context, sdk.DBConfig, int) error   { return nil }
func (s *stubService) RegistryVersion(context.Context, sdk.DBConfig) (int, error) { return 0, nil }

// StartTargetWatcher is a no-op for tests to satisfy the Service interface.
func (s *stubService) StartTargetWatcher(context.Context, sdk.TargetProvider, time.Duration) func() {
	return func() {}
}

func TestLocalClientDelegates(t *testing.T) {
	svc := &stubService{}
	c := client.NewLocalService(svc)
	if c.Mode() != "local" {
		t.Fatalf("mode %s", c.Mode())
	}
	if _, err := c.List(context.Background(), 1, ""); err != nil || !svc.listed {
		t.Fatalf("list")
	}
	if err := c.Create(context.Background(), sdk.FieldMeta{}); err != nil || !svc.created {
		t.Fatalf("create")
	}
	if err := c.Update(context.Background(), sdk.FieldMeta{}); err != nil || !svc.updated {
		t.Fatalf("update")
	}
	if err := c.Delete(context.Background(), "t", "c"); err != nil || !svc.deleted {
		t.Fatalf("delete")
	}
}
