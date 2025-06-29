package client

import (
	"context"
	"fmt"

	sdk "github.com/faciam-dev/gcfm/sdk"
	"github.com/go-resty/resty/v2"
)

// Client provides REST access to the CustomField API.
type Client interface {
	List(ctx context.Context, table string) ([]sdk.FieldMeta, error)
	Create(ctx context.Context, fm sdk.FieldMeta) error
	Update(ctx context.Context, fm sdk.FieldMeta) error
	Delete(ctx context.Context, table, column string) error
}

type client struct {
	base string
	http *resty.Client
}

type Option func(*client)

// WithToken sets the Authorization token
func WithToken(tok string) Option {
	return func(c *client) {
		c.http.SetAuthToken(tok)
	}
}

// New returns a new Client for the given base URL
func New(base string, opts ...Option) Client {
	c := &client{base: base, http: resty.New()}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *client) List(ctx context.Context, table string) ([]sdk.FieldMeta, error) {
	var out []sdk.FieldMeta
	resp, err := c.http.R().SetContext(ctx).SetQueryParam("table", table).SetResult(&out).Get(c.base + "/v1/custom-fields")
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, restyErr(resp)
	}
	return out, nil
}

func (c *client) Create(ctx context.Context, fm sdk.FieldMeta) error {
	body := map[string]any{"table": fm.TableName, "column": fm.ColumnName, "type": fm.DataType}
	resp, err := c.http.R().SetContext(ctx).SetBody(body).Post(c.base + "/v1/custom-fields")
	if err != nil {
		return err
	}
	if resp.IsError() {
		return restyErr(resp)
	}
	return nil
}

func (c *client) Update(ctx context.Context, fm sdk.FieldMeta) error {
	id := fmt.Sprintf("%s.%s", fm.TableName, fm.ColumnName)
	body := map[string]any{"table": fm.TableName, "column": fm.ColumnName, "type": fm.DataType}
	resp, err := c.http.R().SetContext(ctx).SetBody(body).Put(c.base + "/v1/custom-fields/" + id)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return restyErr(resp)
	}
	return nil
}

func (c *client) Delete(ctx context.Context, table, column string) error {
	id := fmt.Sprintf("%s.%s", table, column)
	resp, err := c.http.R().SetContext(ctx).Delete(c.base + "/v1/custom-fields/" + id)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return restyErr(resp)
	}
	return nil
}

func restyErr(resp *resty.Response) error {
	return fmt.Errorf("%s", resp.Status())
}
