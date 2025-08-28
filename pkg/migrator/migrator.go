package migrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/lib/pq"
)

// Migration holds migration data for one version.
type Migration struct {
	Version int
	SemVer  string
	UpSQL   string
	DownSQL string
}

// RegistryMigrator applies migrations for the registry schema.
type RegistryMigrator interface {
	Current(ctx context.Context, db *sql.DB) (int, error)
	Up(ctx context.Context, db *sql.DB, target int) error   // 0=latest
	Down(ctx context.Context, db *sql.DB, target int) error // target<current
}

// Migrator implements RegistryMigrator using embedded SQL.
type Migrator struct {
	migrations  []Migration
	TablePrefix string
	Driver      string
}

func (m *Migrator) versionTable() string {
	return m.TablePrefix + "registry_schema_version"
}

func (m *Migrator) customFieldsTable() string {
	return m.TablePrefix + "custom_fields"
}

// Deprecated: use NewWithDriverAndPrefix(driver, prefix) to avoid implicit prefix errors.
func New() *Migrator {
	log.Println("WARNING: migrator.New() is deprecated; use NewWithDriverAndPrefix")
	return nil
}

// NewWithDriver returns a Migrator for the specified driver.
func NewWithDriver(driver string) *Migrator {
	return NewWithDriverAndPrefix(driver, "")
}

// NewWithDriverAndPrefix returns a Migrator for the driver with table prefix.
func NewWithDriverAndPrefix(driver, prefix string) *Migrator {
	var migs []Migration
	if driver == "postgres" {
		migs = postgresMigrations
	} else {
		migs = defaultMigrations
	}
	migs = withPrefix(migs, prefix)
	return &Migrator{migrations: migs, TablePrefix: prefix, Driver: driver}
}

func withPrefix(migs []Migration, prefix string) []Migration {
	res := make([]Migration, len(migs))
	for i, m := range migs {
		m.UpSQL = strings.ReplaceAll(m.UpSQL, "gcfm_", prefix)
		m.DownSQL = strings.ReplaceAll(m.DownSQL, "gcfm_", prefix)
		res[i] = m
	}
	return res
}

// ErrNoVersionTable indicates gcfm_registry_schema_version table is missing.
var ErrNoVersionTable = errors.New("gcfm_registry_schema_version table not found")

// SemVerToInt converts a semver string to its integer version.
func (m *Migrator) SemVerToInt(v string) (int, bool) {
	for _, mig := range m.migrations {
		if mig.SemVer == v || strings.TrimPrefix(mig.SemVer, "v") == v {
			return mig.Version, true
		}
	}
	return 0, false
}

// Current returns current version (integer). If the version table doesn't exist
// ErrNoVersionTable is returned.
func (m *Migrator) Current(ctx context.Context, db *sql.DB) (int, error) {
	if err := m.ensureVersionTable(ctx, db); err != nil {
		return 0, err
	}
	tbl := m.versionTable()
	var query string
	if m.Driver == "postgres" {
		query = fmt.Sprintf("SELECT MAX(version) FROM %s", pq.QuoteIdentifier(tbl))
	} else {
		query = fmt.Sprintf("SELECT MAX(version) FROM `%s`", tbl)
	}
	row := db.QueryRowContext(ctx, query) // #nosec G201 -- table name derived from trusted prefix
	var v sql.NullInt64
	if err := row.Scan(&v); err != nil {
		if isTableMissing(err) {
			return 0, ErrNoVersionTable
		}
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

func splitSQL(src string) []string {
	var (
		res       []string
		buf       strings.Builder
		inSingle  bool
		inDouble  bool
		dollarTag string
	)
	for i := 0; i < len(src); i++ {
		c := src[i]
		if dollarTag != "" {
			if strings.HasPrefix(src[i:], dollarTag) {
				buf.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			buf.WriteByte(c)
			continue
		}
		switch c {
		case '\'':
			inSingle = !inSingle
		case '"':
			inDouble = !inDouble
		case '$':
			if !inSingle && !inDouble {
				j := i + 1
				for j < len(src) && ((src[j] >= 'a' && src[j] <= 'z') || (src[j] >= 'A' && src[j] <= 'Z') || (src[j] >= '0' && src[j] <= '9') || src[j] == '_') {
					j++
				}
				if j < len(src) && src[j] == '$' {
					dollarTag = src[i : j+1]
					buf.WriteString(dollarTag)
					i = j
					continue
				}
			}
		case ';':
			if !inSingle && !inDouble {
				s := strings.TrimSpace(buf.String())
				if s != "" {
					res = append(res, s)
				}
				buf.Reset()
				continue
			}
		}
		buf.WriteByte(c)
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		res = append(res, s)
	}
	return res
}

func execAll(ctx context.Context, tx *sql.Tx, src string) error {
	for _, stmt := range splitSQL(src) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

func tableExists(ctx context.Context, tx *sql.Tx, name string) bool {
	_, err := tx.ExecContext(ctx, fmt.Sprintf("SELECT 1 FROM %s LIMIT 0", name))
	return err == nil
}

// Up migrates the schema up to target. target=0 means latest.
func (m *Migrator) Up(ctx context.Context, db *sql.DB, target int) error {
	if target == 0 {
		target = len(m.migrations)
	}
	cur, err := m.Current(ctx, db)
	if err != nil {
		return err
	}
	if cur >= target {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for i := cur; i < target; i++ {
		if err := execAll(ctx, tx, m.migrations[i].UpSQL); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return fmt.Errorf("rollback: %v: %w", rbErr, err)
			}
			return err
		}
	}
	return tx.Commit()
}

// Down migrates schema down to target version.
func (m *Migrator) Down(ctx context.Context, db *sql.DB, target int) error {
	cur, err := m.Current(ctx, db)
	if err != nil {
		return err
	}
	if target >= cur {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for i := cur - 1; i >= target; i-- {
		if err := execAll(ctx, tx, m.migrations[i].DownSQL); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return fmt.Errorf("rollback: %v: %w", rbErr, err)
			}
			return err
		}
	}
	return tx.Commit()
}

func isTableMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "no such table") || strings.Contains(msg, "undefined table")
}

// SQLForRange returns SQL statements needed to migrate from->to.
func (m *Migrator) SQLForRange(from, to int) []string {
	var res []string
	if to > from {
		for i := from; i < to; i++ {
			res = append(res, splitSQL(m.migrations[i].UpSQL)...)
		}
	} else if to < from {
		for i := from - 1; i >= to; i-- {
			res = append(res, splitSQL(m.migrations[i].DownSQL)...)
		}
	}
	return res
}
