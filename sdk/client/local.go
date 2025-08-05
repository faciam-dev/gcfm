package client

import (
	"context"

	sdk "github.com/faciam-dev/gcfm/sdk"
)

type localClient struct{ svc sdk.Service }

// NewLocalService wraps an existing sdk.Service as a Client.
func NewLocalService(svc sdk.Service) Client { return &localClient{svc: svc} }

func (l *localClient) List(ctx context.Context, dbID int64, table string) ([]sdk.FieldMeta, error) {
	return l.svc.ListCustomFields(ctx, dbID, table)
}

func (l *localClient) Create(ctx context.Context, fm sdk.FieldMeta) error {
	return l.svc.CreateCustomField(ctx, fm)
}

func (l *localClient) Update(ctx context.Context, fm sdk.FieldMeta) error {
	return l.svc.UpdateCustomField(ctx, fm)
}

func (l *localClient) Delete(ctx context.Context, table, column string) error {
	return l.svc.DeleteCustomField(ctx, table, column)
}

func (l *localClient) Mode() string { return "local" }
