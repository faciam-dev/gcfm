package server

import (
	"database/sql"
	"log"
	"os"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/faciam-dev/gcfm/internal/api/handler"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/go-chi/chi/v5"
)

func New(db *sql.DB, driver, dsn string) huma.API {
	r := chi.NewRouter()

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
	}

	api := humachi.New(r, huma.DefaultConfig("CustomField API", "1.0.0"))
	api.UseMiddleware(middleware.JWT(api, secret))
	if err == nil {
		api.UseMiddleware(middleware.RBAC(e))
	}

	rec := &audit.Recorder{DB: db, Driver: driver}

	handler.Register(api, &handler.CustomFieldHandler{DB: db, Driver: driver, Recorder: rec})
	handler.RegisterRegistry(api, &handler.RegistryHandler{DB: db, Driver: driver, DSN: dsn, Recorder: rec})
	handler.RegisterAudit(api, &handler.AuditHandler{DB: db, Driver: driver})
	return api
}
