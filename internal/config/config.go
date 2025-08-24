package config

import (
	"context"
	"database/sql"
	"fmt"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
)

// Config holds global configuration values.
type Config struct {
	TablePrefix                string   `env:"TABLE_PREFIX,default=gcfm_"`
	PluginsMaxUploadMB         int      `env:"PLUGINS_MAX_UPLOAD_MB,default=20"`
	PluginsTmpDir              string   `env:"PLUGINS_TMP_DIR"`
	PluginsAcceptExt           []string `env:"PLUGINS_ACCEPT_EXT,default=.zip,.tgz,.tar.gz" envSeparator:","`
	PluginsStoreDir            string   `env:"PLUGINS_STORE_DIR"`
	WidgetsNotifyBackend       string   `env:"WIDGETS_NOTIFY_BACKEND,default=redis"`
	RedisURL                   string   `env:"REDIS_URL"`
	WidgetsRedisChannel        string   `env:"WIDGETS_REDIS_CHANNEL,default=widgets_changed"`
	WidgetsRedisReconnectMS    int      `env:"WIDGETS_REDIS_RECONNECT_MS,default=1000"`
	WidgetsRedisReconnectMaxMS int      `env:"WIDGETS_REDIS_RECONNECT_MAX_MS,default=10000"`
}

// T prefixes the given table name with the configured prefix.
func (c *Config) T(name string) string {
	return c.TablePrefix + name
}

// CheckPrefix verifies that tables with the configured prefix exist in the
// connected database. It returns an error if none are found.
func CheckPrefix(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix string) error {
	q := query.New(db, "information_schema.tables", dialect).
		SelectRaw("COUNT(*) as cnt").
		WhereRaw("table_name LIKE :p", map[string]any{"p": prefix + "%"}).
		WithContext(ctx)

	var res struct{ Cnt int }
	if err := q.First(&res); err != nil {
		return err
	}
	if res.Cnt == 0 {
		return fmt.Errorf("no tables with prefix %q found; run migrations or set TABLE_PREFIX correctly", prefix)
	}
	return nil
}
