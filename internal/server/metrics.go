package server

import (
	"context"
	"database/sql"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/pkg/registry"
	"github.com/faciam-dev/gcfm/pkg/metrics"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// setupMetrics registers metrics middleware and handlers.
func setupMetrics(api huma.API, r chi.Router, db *sql.DB, dialect driver.Dialect, tablePrefix string) {
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	api.UseMiddleware(middleware.MetricsMW)
	if db != nil {
		metrics.StartFieldGauge(context.Background(), &registry.Repo{DB: db, Dialect: dialect, TablePrefix: tablePrefix})
	}
}
