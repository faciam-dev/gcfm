package sdk

import "go.uber.org/zap"

// DBConfig specifies database connection parameters.
type DBConfig struct {
	Driver string // mysql|postgres|mongo
	DSN    string
	Schema string
}

// ServiceConfig holds optional configuration for Service.
type ServiceConfig struct {
	Logger    *zap.SugaredLogger
	PluginDir string
}
