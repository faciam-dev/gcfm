package sdk

import (
	"context"

	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
	"github.com/faciam-dev/gcfm/internal/customfield/pluginloader"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// Service exposes high level operations for custom field registry.
// Service provides database operations for custom field registry.
type Service interface {
	// Scan reads schema information from the database.
	Scan(ctx context.Context, cfg DBConfig) ([]registry.FieldMeta, error)
	// Export dumps registry metadata as YAML.
	Export(ctx context.Context, cfg DBConfig) ([]byte, error)
	// Apply updates the registry based on the provided YAML.
	Apply(ctx context.Context, cfg DBConfig, yaml []byte, opts ApplyOptions) (DiffReport, error)
	// MigrateRegistry upgrades or downgrades the registry schema.
	MigrateRegistry(ctx context.Context, cfg DBConfig, target int) error
	// RegistryVersion returns the current registry schema version.
	RegistryVersion(ctx context.Context, cfg DBConfig) (int, error)
}

// New returns a Service initialized with the given configuration.
// Validator plugins under PluginDir are loaded automatically.
func New(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if err := pluginloader.LoadAll(logger); err != nil {
		logger.Errorf("Failed to load plugins: %v", err)
	}
	return &service{logger: logger, pluginDir: cfg.PluginDir, recorder: cfg.Recorder, notifier: cfg.Notifier}
}

type service struct {
	logger    *zap.SugaredLogger
	pluginDir string
	recorder  *audit.Recorder
	notifier  notifier.Broker
}

type ApplyOptions struct {
	// DryRun skips applying changes and only computes the diff.
	DryRun bool
	Actor  string
}

type DiffReport struct {
	// Added is the number of newly created fields.
	Added int
	// Deleted is the number of removed fields.
	Deleted int
	// Updated is the number of modified fields.
	Updated int
}
