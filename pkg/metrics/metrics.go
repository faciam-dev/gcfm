package metrics

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	APIRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_api_requests_total",
			Help: "Number of API requests",
		},
		[]string{"tenant", "method", "path", "status"},
	)
	APILatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cf_api_latency_seconds",
			Help:    "API latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tenant", "method", "path"},
	)
	Fields = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cf_fields_total",
			Help: "Number of custom fields by table",
		},
		[]string{"table"},
	)
	ApplyErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_apply_errors_total",
			Help: "Count of apply errors",
		},
		[]string{"table", "error"},
	)
	CacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cf_cache_hits_total",
			Help: "Runtime cache hits",
		},
	)
	CacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cf_cache_misses_total",
			Help: "Runtime cache misses",
		},
	)
	AuditEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_audit_events_total",
			Help: "Audit log events",
		},
		[]string{"action"},
	)
	AuditErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_audit_errors_total",
			Help: "Audit write errors",
		},
		[]string{"action"},
	)
	Targets = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "cf_targets_total",
			Help: "Number of registered targets",
		},
	)
	TargetLabels = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cf_target_labels_total",
			Help: "Number of targets per label",
		},
		[]string{"label"},
	)
	TargetOpLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cf_target_operation_seconds",
			Help:    "Latency of target registry operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"op", "status"},
	)
	TargetQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cf_target_query_seconds",
			Help:    "Latency of target registry queries",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"op"},
	)
	TargetQueryHits = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cf_target_query_hits",
			Help:    "Number of targets returned by query",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"op"},
	)
	TargetFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cf_target_failures_total",
			Help: "Count of target execution failures",
		},
		[]string{"key", "transient"},
	)
	TargetState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cf_target_breaker_state",
			Help: "Circuit breaker state per target",
		},
		[]string{"key", "state"},
	)
)

func init() {
	prometheus.MustRegister(
		APIRequests,
		APILatency,
		Fields,
		ApplyErrors,
		CacheHits,
		CacheMisses,
		AuditEvents,
		AuditErrors,
		Targets,
		TargetLabels,
		TargetOpLatency,
		TargetQueryLatency,
		TargetQueryHits,
		TargetFailures,
		TargetState,
	)
}

// FieldCounter is implemented by repositories able to count fields per table.
type FieldCounter interface {
	CountFieldsByTable(ctx context.Context) (map[string]int, error)
}

// StartFieldGauge starts a background job that updates the field gauge every 30 seconds.
func StartFieldGauge(ctx context.Context, repo FieldCounter) {
	if repo == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				counts, err := repo.CountFieldsByTable(ctx)
				if err != nil {
					log.Printf("Error in CountFieldsByTable: %v", err)
					continue
				}
				for t, n := range counts {
					Fields.WithLabelValues(t).Set(float64(n))
				}
			}
		}
	}()
}
