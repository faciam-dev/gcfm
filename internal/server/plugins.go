package server

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/handler"
	"github.com/faciam-dev/gcfm/internal/logger"
	widgetsnotify "github.com/faciam-dev/gcfm/internal/notify/widgets"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
	pluginsvc "github.com/faciam-dev/gcfm/internal/service/plugins"
	pluginhandlers "github.com/faciam-dev/gcfm/internal/transport/http/handlers"
	"github.com/faciam-dev/gcfm/internal/util"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// pluginConfig holds environment-driven plugin settings.
type pluginConfig struct {
	MaxUploadMB  int
	TmpDir       string
	StoreDir     string
	AcceptExt    []string
	RedisChannel string
	BackoffMS    int
	BackoffMaxMS int
}

// loadPluginConfig reads plugin-related settings from the environment.
func loadPluginConfig() pluginConfig {
	cfg := pluginConfig{
		MaxUploadMB:  20,
		AcceptExt:    []string{".zip", ".tgz", ".tar.gz"},
		RedisChannel: "widgets_changed",
		BackoffMS:    1000,
		BackoffMaxMS: 10000,
	}
	if v := os.Getenv("PLUGINS_MAX_UPLOAD_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxUploadMB = n
		}
	}
	cfg.TmpDir = os.Getenv("PLUGINS_TMP_DIR")
	cfg.StoreDir = os.Getenv("PLUGINS_STORE_DIR")
	if v := os.Getenv("PLUGINS_ACCEPT_EXT"); v != "" {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		cfg.AcceptExt = parts
	}
	if v := os.Getenv("WIDGETS_REDIS_CHANNEL"); v != "" {
		cfg.RedisChannel = v
	}
	if v := os.Getenv("WIDGETS_REDIS_RECONNECT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BackoffMS = n
		}
	}
	if v := os.Getenv("WIDGETS_REDIS_RECONNECT_MAX_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BackoffMaxMS = n
		}
	}
	return cfg
}

// setupPluginRoutes registers plugin and widget endpoints.
func setupPluginRoutes(api huma.API, r chi.Router, db *sql.DB, driver, tablePrefix string, wreg widgetreg.Registry, e *casbin.Enforcer, resolver func(context.Context, string) ([]string, error)) {
	cfg := loadPluginConfig()
	var wrepo widgetsrepo.Repo
	if db != nil {
		if driver == "postgres" {
			wrepo = widgetsrepo.NewPGRepo(db, tablePrefix)
		} else if driver == "mysql" {
			wrepo = widgetsrepo.NewMySQLRepo(db, tablePrefix)
		}
	}
	var (
		rdb      *redis.Client
		notifier pluginsvc.WidgetsNotifier
	)
	if os.Getenv("WIDGETS_NOTIFY_BACKEND") == "redis" {
		if opt, err := redis.ParseURL(os.Getenv("REDIS_URL")); err == nil {
			rdb = redis.NewClient(opt)
			notifier = widgetsnotify.NewRedisNotifier(rdb, cfg.RedisChannel)
		} else {
			logger.L.Error("parse redis url", "err", err)
		}
	}
	az := authz{Enf: e, Resolve: resolver}
	uploader := &pluginsvc.Uploader{Repo: wrepo, Notifier: notifier, Logger: logger.L, AcceptExt: cfg.AcceptExt, TmpDir: cfg.TmpDir, StoreDir: cfg.StoreDir}
	ph := &pluginhandlers.Handlers{Auth: az, Cfg: pluginhandlers.Config{PluginsMaxUploadMB: cfg.MaxUploadMB}, PluginUploader: uploader}
	ph.RegisterPluginRoutes(api)
	wh := &handler.WidgetHandler{Reg: wreg, Repo: wrepo, Notifier: notifier, Auth: az}
	handler.RegisterWidget(api, wh)
	r.Get("/v1/metadata/widgets/stream", wh.Stream)
	if wrepo != nil {
		rows, _, err := wrepo.List(context.Background(), widgetsrepo.Filter{})
		if err != nil {
			logger.L.Error("load widgets", "err", err)
		} else {
			ws := make([]widgetreg.Widget, len(rows))
			for i, r := range rows {
				ws[i] = widgetreg.Widget{
					ID:           r.ID,
					Name:         r.Name,
					Version:      r.Version,
					Type:         r.Type,
					Scopes:       r.Scopes,
					Enabled:      r.Enabled,
					Description:  util.Deref(r.Description),
					Capabilities: r.Capabilities,
					Homepage:     util.Deref(r.Homepage),
					Meta:         r.Meta,
					Tenants:      r.Tenants,
					UpdatedAt:    r.UpdatedAt,
				}
			}
			wreg.ApplyDiff(context.Background(), ws, nil)
		}
		if rdb != nil {
			sub := &widgetreg.RedisSubscriber{RDB: rdb, Channel: cfg.RedisChannel, Repo: wrepo, Reg: wreg, Logger: logger.L, BackoffMS: cfg.BackoffMS, BackoffMaxMS: cfg.BackoffMaxMS}
			_ = sub.Start(context.Background())
		}
	}
}
