package sdk

import (
	"context"
	"database/sql"
	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
)

// DBConfig specifies database connection parameters.
type DBConfig struct {
	Driver      string // mysql|postgres|mongo
	DSN         string
	Schema      string
	TablePrefix string
}

// ServiceConfig holds optional configuration for Service.
//
// DB, Driver and Schema specify the default connection to the monitored
// database. If the Meta* fields are left nil or empty, they inherit these
// default values.
type ServiceConfig struct {
	Logger          *zap.SugaredLogger
	PluginDir       string
	PluginPublicKey string
	PluginEnabled   *bool
	Recorder        *audit.Recorder
	Notifier        notifier.Broker

	// Default connection for monitored databases. Kept for backward
	// compatibility.
	DB     *sql.DB
	Driver string
	Schema string

	// Optional separate connection for metadata storage. When omitted,
	// the above DB/Driver/Schema values are used.
	MetaDB     *sql.DB
	MetaDriver string
	MetaSchema string

	// Target databases for monitoring. If empty, the default DB/Driver/
	// Schema fields represent the only target.
	Targets []TargetConfig

	// TargetResolver selects a target based on the request context. When
	// nil, operations fall back to the default target.
	TargetResolver TargetResolver
}

// TargetConfig defines an individual monitored database.
type TargetConfig struct {
	Key    string // unique identifier like "tenant:foo" or "db:orders"
	DB     *sql.DB
	Driver string
	Schema string
	Labels []string // optional tags such as "tenant:foo" or "region:tokyo"
}

// TargetResolver chooses a target key from the request context. It returns
// the key and true on success.
type TargetResolver func(ctx context.Context) (key string, ok bool)
