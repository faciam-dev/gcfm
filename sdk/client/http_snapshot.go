package client

import (
	"context"
	"fmt"

	sdk "github.com/faciam-dev/gcfm/sdk"
	"github.com/go-resty/resty/v2"
)

// snapshotHTTP implements sdk.SnapshotClient using HTTP API.
type snapshotHTTP struct {
	base string
	http *resty.Client
}

// Option allows customizing the underlying HTTP client.
// SnapOption allows customizing the underlying HTTP client.
type SnapOption func(*httpClient)

// NewHTTPSnapshot returns a SnapshotClient for HTTP servers.
func NewHTTPSnapshot(base string, opts ...SnapOption) sdk.SnapshotClient {
	tmp := &httpClient{base: base, http: resty.New()}
	for _, o := range opts {
		o(tmp)
	}
	return &snapshotHTTP{base: base, http: tmp.http}
}

func (c *snapshotHTTP) List(ctx context.Context, tenant string) ([]sdk.Snapshot, error) {
	var out []sdk.Snapshot
	resp, err := c.http.R().SetContext(ctx).SetHeader("X-Tenant-ID", tenant).SetResult(&out).Get(c.base + "/v1/snapshots")
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("%s", resp.Status())
	}
	return out, nil
}

func (c *snapshotHTTP) Create(ctx context.Context, tenant, bump, msg string) (sdk.Snapshot, error) {
	body := map[string]any{"bump": bump, "message": msg}
	var out sdk.Snapshot
	resp, err := c.http.R().SetContext(ctx).SetHeader("X-Tenant-ID", tenant).SetBody(body).SetResult(&out).Post(c.base + "/v1/snapshots")
	if err != nil {
		return sdk.Snapshot{}, err
	}
	if resp.IsError() {
		return sdk.Snapshot{}, fmt.Errorf("%s", resp.Status())
	}
	return out, nil
}

func (c *snapshotHTTP) Apply(ctx context.Context, tenant, ver string) error {
	resp, err := c.http.R().SetContext(ctx).SetHeader("X-Tenant-ID", tenant).Post(fmt.Sprintf("%s/v1/snapshots/%s/apply", c.base, ver))
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("%s", resp.Status())
	}
	return nil
}

func (c *snapshotHTTP) Diff(ctx context.Context, tenant, from, to string) (string, error) {
	var out string
	resp, err := c.http.R().SetContext(ctx).SetHeader("X-Tenant-ID", tenant).SetResult(&out).Get(fmt.Sprintf("%s/v1/snapshots/%s/diff/%s", c.base, from, to))
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("%s", resp.Status())
	}
	return out, nil
}

func (c *snapshotHTTP) Mode() string { return "http" }
