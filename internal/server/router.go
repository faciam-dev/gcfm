package server

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/faciam-dev/gcfm/internal/api/handler"
	"github.com/faciam-dev/gcfm/internal/auth"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"runtime"
)

func New(db *sql.DB, driver, dsn string) huma.API {
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

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET environment variable is not set. Application cannot start.")
	}
	m := model.NewModel()
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && r.act == p.act")
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		log.Printf("casbin enforcer: %v", err)
	} else {
		e.AddPolicy("admin", "/v1/*", "GET")
		e.AddPolicy("admin", "/v1/*", "POST")
		e.AddPolicy("admin", "/v1/*", "PUT")
		e.AddPolicy("admin", "/v1/*", "DELETE")
		repo := &auth.UserRepo{DB: db, Driver: driver}
		users, err := repo.List(context.Background())
		if err != nil {
			log.Printf("load users: %v", err)
		} else {
			for _, u := range users {
				e.AddGroupingPolicy(strconv.FormatUint(u.ID, 10), u.Role)
			}
		}
	}

	api := humachi.New(r, huma.DefaultConfig("CustomField API", "1.0.0"))
	jwtHandler := auth.NewJWT(secret, 15*time.Minute)

	// Register login & refresh handlers before applying auth middleware so
	// that they remain publicly accessible.
	auth.Register(api, &auth.Handler{Repo: &auth.UserRepo{DB: db, Driver: driver}, JWT: jwtHandler})

	// Apply authentication & RBAC middleware for the remaining endpoints.
	api.UseMiddleware(auth.Middleware(api, jwtHandler))
	if err == nil {
		api.UseMiddleware(middleware.RBAC(e))
	}

	rec := &audit.Recorder{DB: db, Driver: driver}

	var mongoCli *mongo.Client
	if driver == "mongo" && dsn != "" {
		cli, err := mongo.Connect(context.Background(), options.Client().ApplyURI(dsn))
		if err != nil {
			log.Fatal("Failed to connect to MongoDB: ", err)
		}
		mongoCli = cli
	}

	schema := "public"
	if driver == "mysql" {
		if err := db.QueryRowContext(context.Background(), "SELECT DATABASE()").Scan(&schema); err != nil {
			log.Printf("get schema: %v", err)
		}
	}

	handler.Register(api, &handler.CustomFieldHandler{DB: db, Mongo: mongoCli, Driver: driver, Recorder: rec, Schema: schema})
	handler.RegisterRegistry(api, &handler.RegistryHandler{DB: db, Driver: driver, DSN: dsn, Recorder: rec})
	handler.RegisterAudit(api, &handler.AuditHandler{DB: db, Driver: driver})
	handler.RegisterMetadata(api, &handler.MetadataHandler{DB: db, Driver: driver})
	return api
}
