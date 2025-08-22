package server

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	admintargets "github.com/faciam-dev/gcfm/internal/adminapi/targets"
	"github.com/faciam-dev/gcfm/internal/api/handler"
	"github.com/faciam-dev/gcfm/internal/auth"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/events"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/metrics"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/plugin"
	"github.com/faciam-dev/gcfm/internal/plugin/fsrepo"
	"github.com/faciam-dev/gcfm/internal/rbac"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/internal/server/roles"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"github.com/faciam-dev/gcfm/meta/sqlmetastore"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func New(db *sql.DB, cfg DBConfig) huma.API {
	r := chi.NewRouter()

	_, file, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(file), "..", "..")
	reserved.Load(filepath.Join(base, "configs", "default.yaml"))
	allowed := os.Getenv("ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:5173"
	}
	origins := strings.Split(allowed, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	driver := cfg.Driver
	dsn := cfg.DSN
	var dialect ormdriver.Dialect
	if driver == "postgres" {
		dialect = ormdriver.PostgresDialect{}
	} else {
		dialect = ormdriver.MySQLDialect{}
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		logger.L.Error("JWT_SECRET environment variable is not set. Application cannot start.")
		os.Exit(1)
	}
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && r.act == p.act")
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		logger.L.Error("casbin enforcer", "err", err)
	} else {
		e.AddPolicy("admin", "/v1/*", "GET")
		e.AddPolicy("admin", "/v1/*", "POST")
		e.AddPolicy("admin", "/v1/*", "PUT")
		e.AddPolicy("admin", "/v1/*", "DELETE")
		e.AddPolicy("admin", "/v1/audit-logs/*/diff", "GET")
		// Allow admin role to manage target configuration
		e.AddPolicy("admin", "/admin/*", "GET")
		e.AddPolicy("admin", "/admin/*", "POST")
		e.AddPolicy("admin", "/admin/*", "PUT")
		e.AddPolicy("admin", "/admin/*", "DELETE")
		if db != nil {
			if err := rbac.Load(context.Background(), db, dialect, cfg.TablePrefix, e); err != nil {
				logger.L.Error("load rbac", "err", err)
			}
		}
	}

	api := humachi.New(r, huma.DefaultConfig("CustomField API", "1.0.0"))
	jwtHandler := auth.NewJWT(secret, 15*time.Minute)

	// Apply tenant middleware to all endpoints, including login.
	api.UseMiddleware(middleware.ExtractTenant(api))

	// Register login & refresh handlers before applying auth middleware so
	// that they remain publicly accessible.
	auth.Register(api, &auth.Handler{Repo: &auth.UserRepo{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix}, JWT: jwtHandler})

	// Apply authentication middleware for subsequent endpoints.
	api.UseMiddleware(auth.Middleware(api, jwtHandler))

	// ---- role resolver used by RBAC and capabilities ----
	resolver := func(ctx context.Context, user string) ([]string, error) {
		tid := tenant.FromContext(ctx)
		return roles.OfUser(ctx, db, dialect, cfg.TablePrefix, user, tid)
	}

	// Register authenticated capability endpoint before RBAC enforcement.
	handler.RegisterAuthCaps(api, &handler.AuthHandler{Enf: e, DB: db, Driver: driver, TablePrefix: cfg.TablePrefix})

	// Apply RBAC middleware for the remaining endpoints.
	if err == nil {
		api.UseMiddleware(middleware.RBAC(e, resolver))
	}
	api.UseMiddleware(middleware.MetricsMW)

	rec := &audit.Recorder{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix}

	evtConf, err := events.LoadConfig(os.Getenv("CF_EVENTS_CONFIG"))
	if err != nil {
		logger.L.Error("Failed to load events configuration", "err", err)
		os.Exit(1)
	}
	var sinks []events.Sink
	if wh := events.NewWebhookSink(evtConf.Sinks.Webhook); wh != nil {
		sinks = append(sinks, wh)
	}
	if rs, err := events.NewRedisSink(evtConf.Sinks.Redis); err == nil && rs != nil {
		sinks = append(sinks, rs)
	} else if err != nil {
		logger.L.Error("redis sink", "err", err)
	}
	if ks, err := events.NewKafkaSink(evtConf.Sinks.Kafka); err == nil && ks != nil {
		sinks = append(sinks, ks)
	} else if err != nil {
		logger.L.Error("kafka sink", "err", err)
	}
	events.Default = events.NewDispatcher(evtConf, &events.SQLDLQ{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix}, sinks...)
	var mongoCli *mongo.Client
	if driver == "mongo" && dsn != "" {
		cli, err := mongo.Connect(context.Background(), options.Client().ApplyURI(dsn))
		if err != nil {
			logger.L.Error("Failed to connect to MongoDB", "err", err)
			os.Exit(1)
		}
		mongoCli = cli
	}

	schema := "public"
	if driver == "mysql" {
		if err := db.QueryRowContext(context.Background(), "SELECT DATABASE()").Scan(&schema); err != nil {
			logger.L.Error("get schema", "err", err)
		}
	}

	handler.Register(api, &handler.CustomFieldHandler{DB: db, Mongo: mongoCli, Driver: driver, Dialect: dialect, Recorder: rec, Schema: schema, TablePrefix: cfg.TablePrefix})
	handler.RegisterRegistry(api, &handler.RegistryHandler{DB: db, Driver: driver, DSN: dsn, Recorder: rec, TablePrefix: cfg.TablePrefix})
	handler.RegisterSnapshot(api, &handler.SnapshotHandler{DB: db, Driver: driver, Dialect: dialect, DSN: dsn, Recorder: rec, TablePrefix: cfg.TablePrefix})
	handler.RegisterAudit(api, &handler.AuditHandler{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix})
	handler.RegisterRBAC(api, &handler.RBACHandler{DB: db, Dialect: dialect, PasswordCost: bcrypt.DefaultCost, TablePrefix: cfg.TablePrefix, Recorder: rec})
	handler.RegisterMetadata(api, &handler.MetadataHandler{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix})
	handler.RegisterDatabase(api, &handler.DatabaseHandler{Repo: &monitordb.Repo{DB: db, Driver: driver, Dialect: dialect, TablePrefix: cfg.TablePrefix}, Recorder: rec, Enf: e})
	handler.RegisterPlugins(api, &handler.PluginHandler{UC: plugin.Usecase{Repo: &fsrepo.Repository{}}})
	wreg := widgetreg.NewInMemory()
	var wrepo widgetsrepo.Repo
	if db != nil && driver == "postgres" {
		wrepo = widgetsrepo.NewPGRepo(db)
	}
	wh := &handler.WidgetHandler{Reg: wreg, Repo: wrepo}
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
					Description:  derefPtr(r.Description),
					Capabilities: r.Capabilities,
					Homepage:     derefPtr(r.Homepage),
					Meta:         r.Meta,
					Tenants:      r.Tenants,
					UpdatedAt:    r.UpdatedAt,
				}
			}
			wreg.ApplyDiff(context.Background(), ws, nil)
		}
		listener := widgetreg.NewPGListener(dsn, wrepo, wreg, logger.L)
		if _, err := listener.Start(context.Background()); err != nil {
			logger.L.Error("start widget listener", "err", err)
		}
	}
	// simple scope middleware placeholder; integrates with JWT claims if available
	scope := func(scopes ...string) func(huma.Context, func(huma.Context)) {
		return func(ctx huma.Context, next func(huma.Context)) {
			next(ctx)
		}
	}
	admintargets.RegisterRoutes(api, admintargets.Deps{Meta: sqlmetastore.NewSQLMetaStore(db, driver, schema), Rec: rec, Auth: scope})
	if db != nil {
		metrics.StartFieldGauge(context.Background(), &registry.Repo{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix})
	}
	return api
}

func derefPtr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}
