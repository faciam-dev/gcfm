package sdk

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/notifier"
	"github.com/faciam-dev/gcfm/internal/customfield/pluginloader"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	metapkg "github.com/faciam-dev/gcfm/meta"
	"github.com/faciam-dev/gcfm/meta/sqlmetastore"
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
	// ListCustomFields returns custom field metadata.
	ListCustomFields(ctx context.Context, dbID int64, table string) ([]registry.FieldMeta, error)
	// CreateCustomField inserts a new field into the registry.
	CreateCustomField(ctx context.Context, fm registry.FieldMeta) error
	// UpdateCustomField modifies an existing field.
	UpdateCustomField(ctx context.Context, fm registry.FieldMeta) error
	// DeleteCustomField removes a field from the registry.
	DeleteCustomField(ctx context.Context, table, column string) error
}

// New returns a Service initialized with the given configuration.
// Validator plugins under PluginDir are loaded automatically.
func New(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	pluginloader.PublicKeyPath = cfg.PluginPublicKey
	enabled := true
	if cfg.PluginEnabled != nil {
		enabled = *cfg.PluginEnabled
	}
	pluginloader.Enabled = enabled
	if err := pluginloader.LoadAll(cfg.PluginDir, logger); err != nil {
		logger.Errorf("Failed to load plugins from %s: %v", cfg.PluginDir, err)
	}

	metaDB := cfg.MetaDB
	if metaDB == nil {
		metaDB = cfg.DB
	}
	metaDriver := cfg.MetaDriver
	if metaDriver == "" {
		metaDriver = cfg.Driver
	}
	metaSchema := cfg.MetaSchema
	if metaSchema == "" {
		metaSchema = cfg.Schema
	}

	var def TargetConn
	if cfg.DB != nil {
		def = TargetConn{DB: cfg.DB, Driver: cfg.Driver, Schema: cfg.Schema}
	}
	reg := NewTargetRegistry(def)
	for _, t := range cfg.Targets {
		drv := t.Driver
		if drv == "" {
			drv = cfg.Driver
		}
		sch := t.Schema
		if sch == "" {
			sch = cfg.Schema
		}
		if t.DB == nil {
			continue
		}
		_ = reg.Register(TargetConfig{Key: t.Key, DB: t.DB, Driver: drv, Schema: sch, Labels: t.Labels})
	}

	return &service{
		logger:    logger,
		pluginDir: cfg.PluginDir,
		recorder:  cfg.Recorder,
		notifier:  cfg.Notifier,
		meta:      sqlmetastore.NewSQLMetaStore(metaDB, metaDriver, metaSchema),
		targets:   reg,
		resolve:   cfg.TargetResolver,
	}
}

type service struct {
	logger    *zap.SugaredLogger
	pluginDir string
	recorder  *audit.Recorder
	notifier  notifier.Broker
	meta      metapkg.Store
	targets   TargetRegistry
	resolve   TargetResolver
}

var ErrNoTarget = errors.New("no target database resolved")

func (s *service) pickTarget(ctx context.Context) (TargetConn, error) {
	if s.resolve != nil {
		if key, ok := s.resolve(ctx); ok {
			if t, ok := s.targets.Get(key); ok {
				return t, nil
			}
		}
	}
	if t, ok := s.targets.Default(); ok {
		return t, nil
	}
	return TargetConn{}, ErrNoTarget
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
