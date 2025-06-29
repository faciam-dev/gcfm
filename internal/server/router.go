package server

import (
	"database/sql"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/faciam-dev/gcfm/internal/api/handler"
	"github.com/go-chi/chi/v5"
)

func New(db *sql.DB, driver, dsn string) huma.API {
	r := chi.NewRouter()
	api := humachi.New(r, huma.DefaultConfig("CustomField API", "1.0.0"))
	handler.Register(api, &handler.CustomFieldHandler{DB: db, Driver: driver})
	handler.RegisterRegistry(api, &handler.RegistryHandler{DB: db, Driver: driver, DSN: dsn})
	handler.RegisterAudit(api, &handler.AuditHandler{DB: db, Driver: driver})
	return api
}
