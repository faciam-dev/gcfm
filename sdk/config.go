package sdk

import (
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
}
