package middleware

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/faciam-dev/gcfm/internal/metrics"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

// MetricsMW records API request metrics.
func MetricsMW(ctx huma.Context, next func(huma.Context)) {
	r, w := humachi.Unwrap(ctx)
	m := httpsnoop.CaptureMetricsFn(w, func(w http.ResponseWriter) {
		next(humachi.NewContext(ctx.Operation(), r, w))
	})
	normalizedPath := normalizePath(r.URL.Path)
	tid := tenant.FromContext(r.Context())
	labels := prometheus.Labels{"tenant": tid, "method": r.Method, "path": normalizedPath, "status": strconv.Itoa(m.Code)}
	metrics.APIRequests.With(labels).Inc()
	metrics.APILatency.WithLabelValues(r.Method, normalizedPath).Observe(m.Duration.Seconds())
}

var idRe = regexp.MustCompile(`\d+`)

func normalizePath(path string) string {
	return idRe.ReplaceAllString(path, ":id")
}
