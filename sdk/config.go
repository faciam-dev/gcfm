package sdk

import (
	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
)

// DBConfig specifies database connection parameters.
type DBConfig struct {
	Driver string // mysql|postgres|mongo
	DSN    string
	Schema string
}

// ServiceConfig holds optional configuration for Service.
type ServiceConfig struct {
	Logger          *zap.SugaredLogger
	PluginDir       string
	PluginPublicKey string
	PluginEnabled   *bool
	Recorder        *audit.Recorder
	Notifier        notifier.Broker
}
