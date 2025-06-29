package cache

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield/pluginloader"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// scannerFunc wraps a registry.Scanner with its DBConfig
// so Cache can call Scan without parameters.
type scannerFunc func(ctx context.Context) ([]registry.FieldMeta, error)

type Cache struct {
	mu      sync.RWMutex
	byTable map[string]map[string]registry.FieldMeta
}

func New(ctx context.Context, scan registry.Scanner, conf registry.DBConfig, reloadInterval time.Duration) (*Cache, error) {
	logger := zap.NewNop().Sugar()
	if err := pluginloader.LoadAll(logger); err != nil {
		return nil, err
	}
	fn := func(c context.Context) ([]registry.FieldMeta, error) { return scan.Scan(c, conf) }
	return newWithFunc(ctx, fn, reloadInterval)
}

func newWithFunc(ctx context.Context, fn scannerFunc, interval time.Duration) (*Cache, error) {
	metas, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	c := &Cache{byTable: group(metas)}
	if interval > 0 {
		go c.start(ctx, fn, interval)
	}
	return c, nil
}

func (c *Cache) start(ctx context.Context, fn scannerFunc, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metas, err := fn(ctx)
			if err != nil {
				continue
			}
			m := group(metas)
			c.mu.Lock()
			c.byTable = m
			c.mu.Unlock()
		}
	}
}

func group(metas []registry.FieldMeta) map[string]map[string]registry.FieldMeta {
	res := make(map[string]map[string]registry.FieldMeta)
	for _, m := range metas {
		tbl := res[m.TableName]
		if tbl == nil {
			tbl = make(map[string]registry.FieldMeta)
			res[m.TableName] = tbl
		}
		tbl[m.ColumnName] = m
	}
	return res
}

func (c *Cache) Field(table, column string) (registry.FieldMeta, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cols, ok := c.byTable[table]
	if !ok {
		return registry.FieldMeta{}, false
	}
	m, ok := cols[column]
	return m, ok
}

func (c *Cache) Table(table string) map[string]registry.FieldMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	src, ok := c.byTable[table]
	if !ok {
		return nil
	}
	dst := make(map[string]registry.FieldMeta, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
