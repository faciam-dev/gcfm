package server

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	admintargets "github.com/faciam-dev/gcfm/internal/adminapi/targets"
	"github.com/faciam-dev/gcfm/internal/api/handler"
	"github.com/faciam-dev/gcfm/internal/auth"
	capabilitydomain "github.com/faciam-dev/gcfm/internal/domain/capability"
	capmongoadapter "github.com/faciam-dev/gcfm/internal/infrastructure/capability/mongo"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/plugin"
	"github.com/faciam-dev/gcfm/internal/plugin/fsrepo"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	capabilityusecase "github.com/faciam-dev/gcfm/internal/usecase/capability"
	"github.com/faciam-dev/gcfm/meta/sqlmetastore"
	"github.com/faciam-dev/gcfm/pkg/audit"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
	"github.com/faciam-dev/gcfm/pkg/widgetpolicy"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func New(db *sql.DB, cfg DBConfig) huma.API {
	r := chi.NewRouter()

	_, file, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(file), "..", "..")
	reserved.Load(filepath.Join(base, "configs", "default.yaml"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	driver := cfg.Driver
	dsn := cfg.DSN
	dialect := pkgutil.DialectFromDriver(driver)
	secret := mustJWTSecret()
	e, err := initEnforcer(db, dialect, cfg.TablePrefix)
	if err != nil {
		logger.L.Error("casbin enforcer", "err", err)
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
	resolver := roleResolver(db, dialect, cfg.TablePrefix)

	// Register authenticated capability endpoint before RBAC enforcement.
	handler.RegisterAuthCaps(api, &handler.AuthHandler{Enf: e, DB: db, Driver: driver, TablePrefix: cfg.TablePrefix})

	// Apply RBAC middleware for the remaining endpoints.
	if err == nil {
		api.UseMiddleware(middleware.RBAC(e, resolver))
	}
	setupMetrics(api, r, db, dialect, cfg.TablePrefix)

	rec := &audit.Recorder{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix}

	initEvents(db, dialect, cfg.TablePrefix)
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

	wreg := widgetreg.NewInMemory()
	policyPath := os.Getenv("WIDGET_POLICY_PATH")
	if policyPath == "" {
		policyPath = filepath.Join("configs", "widget_policies.yml")
	}
	wpStore := widgetpolicy.NewStore(policyPath, logger.L)
	if err := wpStore.Load(); err != nil {
		logger.L.Warn("load widget policy", "err", err)
	}
	go wpStore.Watch(context.Background())
	handler.Register(api, &handler.CustomFieldHandler{DB: db, Mongo: mongoCli, Driver: driver, Dialect: dialect, Recorder: rec, Schema: schema, TablePrefix: cfg.TablePrefix, WidgetRegistry: wreg, PolicyStore: wpStore})
	handler.RegisterWidgetPolicy(api, &handler.WidgetPolicyHandler{Store: wpStore, Registry: wreg, PolicyPath: policyPath})
	handler.RegisterCustomFieldValidators(api)
	handler.RegisterRegistry(api, &handler.RegistryHandler{DB: db, Driver: driver, DSN: dsn, Recorder: rec, TablePrefix: cfg.TablePrefix})
	handler.RegisterSnapshot(api, &handler.SnapshotHandler{DB: db, Driver: driver, Dialect: dialect, DSN: dsn, Recorder: rec, TablePrefix: cfg.TablePrefix})
	handler.RegisterAudit(api, &handler.AuditHandler{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix})
	handler.RegisterRBAC(api, &handler.RBACHandler{DB: db, Dialect: dialect, PasswordCost: bcrypt.DefaultCost, TablePrefix: cfg.TablePrefix, Recorder: rec})
	handler.RegisterMetadata(api, &handler.MetadataHandler{DB: db, Dialect: dialect, TablePrefix: cfg.TablePrefix})
	dbRepo := &monitordb.Repo{DB: db, Driver: driver, Dialect: dialect, TablePrefix: cfg.TablePrefix}
	mongoCapAdapter := capmongoadapter.New()
	capAdapters := map[string]capabilitydomain.Adapter{
		"mongo":   mongoCapAdapter,
		"mongodb": mongoCapAdapter,
	}
	capSvc := capabilityusecase.New(dbRepo, capAdapters)
	handler.RegisterDatabase(api, &handler.DatabaseHandler{Repo: dbRepo, Recorder: rec, Enf: e, Capabilities: capSvc})
	handler.RegisterPlugins(api, &handler.PluginHandler{UC: plugin.Usecase{Repo: &fsrepo.Repository{}}})

	setupPluginRoutes(api, r, db, driver, cfg.TablePrefix, wreg, e, resolver)
	// simple scope middleware placeholder; integrates with JWT claims if available
	scope := func(scopes ...string) func(huma.Context, func(huma.Context)) {
		return func(ctx huma.Context, next func(huma.Context)) {
			next(ctx)
		}
	}
	admintargets.RegisterRoutes(api, admintargets.Deps{Meta: sqlmetastore.NewSQLMetaStore(db, driver, schema), Rec: rec, Auth: scope})
	return api
}

type authz struct {
	Enf     *casbin.Enforcer
	Resolve func(context.Context, string) ([]string, error)
}

func (a authz) HasCapability(ctx context.Context, capKey string) bool {
	if a.Enf == nil {
		return false
	}
	capDef, ok := handler.CapabilityByKey(capKey)
	if !ok {
		return false
	}
	user := middleware.UserFromContext(ctx)
	subjects := []string{user}
	if a.Resolve != nil {
		if roles, err := a.Resolve(ctx, user); err == nil {
			subjects = append(subjects, roles...)
		}
	}
	for _, s := range subjects {
		if ok, _ := a.Enf.Enforce(s, capDef.Path, capDef.Method); ok {
			return true
		}
	}
	return false
}
