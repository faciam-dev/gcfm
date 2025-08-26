package sdk

import (
	"context"
	"database/sql"
	"go.uber.org/zap"
	"time"

	"github.com/faciam-dev/gcfm/pkg/audit"
	"github.com/faciam-dev/gcfm/pkg/notifier"
)

// DBConfig specifies database connection parameters.
type DBConfig struct {
	Driver      string // mysql|postgres|mongo
	DSN         string
	Schema      string
	TablePrefix string
}

// Connector is responsible for establishing physical database connections.
type Connector func(ctx context.Context, driver, dsnOrURL string) (*sql.DB, error)

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

	// TargetResolverV2 returns either a direct key or a query. When both
	// V1 and V2 are provided, V2 takes precedence.
	TargetResolverV2 TargetResolverV2

	// DefaultStrategy determines how to choose among multiple candidates
	// returned by a query. The zero value means SelectFirst.
	DefaultStrategy SelectionStrategy

	// DefaultPreferLabel is used when DefaultStrategy is SelectPreferLabel
	// and no hint is supplied.
	DefaultPreferLabel string

	// Connector creates new DB connections. When nil, a default
	// implementation based on sql.Open and PingContext is used.
	Connector Connector

	// Failover controls retry and circuit breaker behavior.
	Failover FailoverPolicy

	// ErrorClassifier distinguishes transient errors for retry decisions.
	ErrorClassifier ErrorClassifier

	// ReadSource selects where to read custom field metadata from.
	ReadSource ReadSource
}

// TargetConfig defines an individual monitored database.
type TargetConfig struct {
	Key    string // unique identifier like "tenant:foo" or "db:orders"
	Driver string
	Schema string
	Labels []string // optional tags such as "tenant:foo" or "region:tokyo"

	// Physical connection information (hot reload target).
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxIdle  time.Duration
	ConnMaxLife  time.Duration

	// Backward compatibility: pre-established connection. Connections
	// provided via DB are not subject to hot reload.
	DB *sql.DB
}

// TargetResolver chooses a target key from the request context. It returns
// the key and true on success.
type TargetResolver func(ctx context.Context) (key string, ok bool)

// TargetDecision represents a proposal for selecting a target. Either Key or
// Query (or both) may be specified.
type TargetDecision struct {
	Key   string
	Query *Query
	Hint  *SelectionHint
}

// TargetResolverV2 returns a TargetDecision derived from the request context.
// It returns false when no decision could be made.
type TargetResolverV2 func(ctx context.Context) (TargetDecision, bool)

// SelectionStrategy indicates how to choose a key from multiple matches.
type SelectionStrategy int

const (
	// SelectFirst picks the first key in sorted order.
	SelectFirst SelectionStrategy = iota
	// SelectPreferLabel prioritizes targets with a given label.
	SelectPreferLabel
	// SelectConsistentHash chooses a target based on a consistent hash of
	// a provided source string.
	SelectConsistentHash
)

// SelectionHint provides optional parameters for selection strategies.
type SelectionHint struct {
	Strategy    SelectionStrategy
	PreferLabel string
	HashSource  string
}
