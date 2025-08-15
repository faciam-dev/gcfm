package sdk

import (
	"context"

	metapkg "github.com/faciam-dev/gcfm/meta"
)

// MetaDBProvider reads target configuration from a MetaStore.
type MetaDBProvider struct {
	meta metapkg.MetaStore
}

// NewMetaDBProvider creates a provider backed by MetaStore.
func NewMetaDBProvider(meta metapkg.MetaStore) *MetaDBProvider { return &MetaDBProvider{meta: meta} }

// Fetch retrieves target configurations from the meta store.
func (p *MetaDBProvider) Fetch(ctx context.Context) (map[string]TargetConfig, string, string, error) {
	rows, ver, def, err := p.meta.ListTargets(ctx)
	if err != nil {
		return nil, "", "", err
	}
	cfgs := make(map[string]TargetConfig, len(rows))
	for _, r := range rows {
		cfgs[r.Key] = TargetConfig{
			Key:          r.Key,
			Driver:       r.Driver,
			DSN:          r.DSN,
			Schema:       r.Schema,
			MaxOpenConns: r.MaxOpenConns,
			MaxIdleConns: r.MaxIdleConns,
			ConnMaxIdle:  r.ConnMaxIdle,
			ConnMaxLife:  r.ConnMaxLife,
			Labels:       r.Labels,
		}
	}
	return cfgs, def, ver, nil
}
