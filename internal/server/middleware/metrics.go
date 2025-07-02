package middleware

import (
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/faciam-dev/gcfm/internal/metrics"
)

// MetricsMW records API request metrics.
func MetricsMW(ctx huma.Context, next func(huma.Context)) {
	r, w := humachi.Unwrap(ctx)
	m := httpsnoop.CaptureMetricsFn(w, func(w http.ResponseWriter) {
		next(humachi.NewContext(ctx.Operation(), r, w))
	})
	labels := prometheus.Labels{"method": r.Method, "path": r.URL.Path, "status": strconv.Itoa(m.Code)}
	metrics.APIRequests.With(labels).Inc()
	metrics.APILatency.WithLabelValues(r.Method, r.URL.Path).Observe(m.Duration.Seconds())
}
