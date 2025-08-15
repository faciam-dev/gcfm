package sdk

import (
	"context"
	"errors"
	"hash/fnv"
	"sort"
	"time"

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
	// StartTargetWatcher periodically fetches target configurations from a provider.
	StartTargetWatcher(ctx context.Context, p TargetProvider, interval time.Duration) (stop func())
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

	var def *TargetConn
	if cfg.DB != nil {
		def = &TargetConn{DB: cfg.DB, Driver: cfg.Driver, Schema: cfg.Schema}
	}
	reg := NewHotReloadRegistry(def)
	mk := cfg.Connector
	if mk == nil {
		mk = defaultConnector
	}
	for _, t := range cfg.Targets {
		drv := t.Driver
		if drv == "" {
			drv = cfg.Driver
		}
		sch := t.Schema
		if sch == "" {
			sch = cfg.Schema
		}
		tc := TargetConfig{
			Driver:       drv,
			Schema:       sch,
			Labels:       t.Labels,
			DSN:          t.DSN,
			MaxOpenConns: t.MaxOpenConns,
			MaxIdleConns: t.MaxIdleConns,
			ConnMaxIdle:  t.ConnMaxIdle,
			ConnMaxLife:  t.ConnMaxLife,
			DB:           t.DB,
		}
		if err := reg.Register(context.Background(), t.Key, tc, mk); err != nil {
			logger.Errorf("Failed to register target %s: %v", t.Key, err)
		}
	}

	return &service{
		logger:       logger,
		pluginDir:    cfg.PluginDir,
		recorder:     cfg.Recorder,
		notifier:     cfg.Notifier,
		meta:         sqlmetastore.NewSQLMetaStore(metaDB, metaDriver, metaSchema),
		targets:      reg,
		resolveV1:    cfg.TargetResolver,
		resolveV2:    cfg.TargetResolverV2,
		stratDefault: cfg.DefaultStrategy,
		stratPrefer:  cfg.DefaultPreferLabel,
		cn:           cfg.Connector,
	}
}

type service struct {
	logger       *zap.SugaredLogger
	pluginDir    string
	recorder     *audit.Recorder
	notifier     notifier.Broker
	meta         metapkg.MetaStore
	targets      TargetRegistry
	resolveV1    TargetResolver
	resolveV2    TargetResolverV2
	stratDefault SelectionStrategy
	stratPrefer  string
	cn           Connector
}

var ErrNoTarget = errors.New("no target database resolved")

// pickTarget resolves a target connection in priority order:
//  1. V2 resolver explicit Key
//  2. V2 resolver Query with selection strategy
//  3. Legacy V1 resolver
//  4. Registry default
//
// For auditing, callers may log the decision (chosen key, collected labels,
// strategy, hash source, number of candidates) at DEBUG level.
func (s *service) pickTarget(ctx context.Context) (TargetConn, error) {
	if s.resolveV2 != nil {
		if dec, ok := s.resolveV2(ctx); ok {
			if dec.Key != "" {
				if t, ok := s.targets.Get(dec.Key); ok {
					return t, nil
				}
			}
			if dec.Query != nil {
				keys := s.targets.FindByQuery(*dec.Query)
				if key, ok := s.chooseOne(keys, dec.Hint); ok {
					if t, ok := s.targets.Get(key); ok {
						return t, nil
					}
				}
			}
		}
	}
	if s.resolveV1 != nil {
		if key, ok := s.resolveV1(ctx); ok {
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

func (s *service) chooseOne(keys []string, hint *SelectionHint) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	sort.Strings(keys)

	strategy := s.stratDefault
	prefer := s.stratPrefer
	var hashSrc string
	if hint != nil {
		if hint.Strategy != 0 {
			strategy = hint.Strategy
		}
		if hint.PreferLabel != "" {
			prefer = hint.PreferLabel
		}
		if hint.HashSource != "" {
			hashSrc = hint.HashSource
		}
	}

	switch strategy {
	case SelectFirst:
		return keys[0], true
	case SelectPreferLabel:
		preferKeys := s.targets.FindByLabel(prefer)
		if len(preferKeys) > 0 {
			set := make(map[string]struct{}, len(preferKeys))
			for _, k := range preferKeys {
				set[k] = struct{}{}
			}
			var filtered []string
			for _, k := range keys {
				if _, ok := set[k]; ok {
					filtered = append(filtered, k)
				}
			}
			if len(filtered) > 0 {
				sort.Strings(filtered)
				return filtered[0], true
			}
		}
		return keys[0], true
	case SelectConsistentHash:
		if hashSrc == "" {
			hashSrc = keys[0]
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(hashSrc))
		idx := int(h.Sum32() % uint32(len(keys)))
		return keys[idx], true
	default:
		return keys[0], true
	}
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
