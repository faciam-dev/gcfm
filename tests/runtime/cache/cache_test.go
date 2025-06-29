package cache_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	runtimecache "github.com/faciam-dev/gcfm/internal/customfield/runtime/cache"
)

type fakeScanner struct {
	metas atomic.Value
}

func (f *fakeScanner) Scan(ctx context.Context, _ registry.DBConfig) ([]registry.FieldMeta, error) {
	v := f.metas.Load().([]registry.FieldMeta)
	return v, nil
}

func TestCacheLookup(t *testing.T) {
	fs := &fakeScanner{}
	fs.metas.Store([]registry.FieldMeta{{TableName: "t", ColumnName: "c", DataType: "int"}})
	c, err := runtimecache.New(context.Background(), fs, registry.DBConfig{}, 0)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, ok := c.Field("t", "c"); !ok {
		t.Fatalf("missing")
	}
	tbl := c.Table("t")
	if len(tbl) != 1 {
		t.Fatalf("table size")
	}
}

func TestCacheReload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fs := &fakeScanner{}
	fs.metas.Store([]registry.FieldMeta{{TableName: "t", ColumnName: "c", DataType: "int"}})
	c, err := runtimecache.New(ctx, fs, registry.DBConfig{}, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	fs.metas.Store([]registry.FieldMeta{{TableName: "t", ColumnName: "d", DataType: "int"}})
	time.Sleep(15 * time.Millisecond)
	if _, ok := c.Field("t", "d"); !ok {
		t.Fatalf("reload failed")
	}
}
