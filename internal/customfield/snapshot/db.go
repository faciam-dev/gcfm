package snapshot

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
	"gopkg.in/yaml.v3"
)

type Record struct {
	ID      int64
	Semver  string
	YAML    []byte
	TakenAt time.Time
	Author  string
}

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

type SnapshotData struct {
	Tenant string
	Semver string
	Author string
	YAML   []byte
}

func Insert(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix string, data SnapshotData) (Record, error) {
	table := prefix + "registry_snapshots"
	if _, ok := dialect.(ormdriver.PostgresDialect); ok {
		stmt := fmt.Sprintf("INSERT INTO %s(tenant_id, semver, yaml, author) VALUES($1,$2,$3,$4) RETURNING id, taken_at", table)
		var (
			id int64
			ts time.Time
		)
		if err := db.QueryRowContext(ctx, stmt, data.Tenant, data.Semver, data.YAML, data.Author).Scan(&id, &ts); err != nil {
			return Record{}, err
		}
		return Record{ID: id, Semver: data.Semver, YAML: data.YAML, TakenAt: ts, Author: data.Author}, nil
	}
	id, err := query.New(db, table, dialect).WithContext(ctx).InsertGetId(map[string]any{
		"tenant_id": data.Tenant,
		"semver":    data.Semver,
		"yaml":      data.YAML,
		"author":    data.Author,
	})
	if err != nil {
		return Record{}, err
	}
	var ts struct {
		TakenAt time.Time `db:"taken_at"`
	}
	if err := query.New(db, table, dialect).Select("taken_at").Where("id", id).WithContext(ctx).First(&ts); err != nil {
		return Record{}, err
	}
	return Record{ID: id, Semver: data.Semver, YAML: data.YAML, TakenAt: ts.TakenAt, Author: data.Author}, nil
}

func Get(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix, tenant, ver string) (Record, error) {
	table := prefix + "registry_snapshots"
	q := query.New(db, table, dialect).
		Select("id", "semver", "yaml", "taken_at", "author").
		Where("tenant_id", tenant).
		Where("semver", ver).
		WithContext(ctx)

	var r Record
	if err := q.First(&r); err != nil {
		return r, err
	}
	return r, nil
}

func List(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix, tenant string, limit int) ([]Record, error) {
	if limit == 0 {
		limit = 20
	}
	table := prefix + "registry_snapshots"
	q := query.New(db, table, dialect).
		Select("id", "semver", "taken_at", "author").
		Where("tenant_id", tenant).
		OrderBy("id", "desc").
		Limit(limit).
		WithContext(ctx)

	var rows []Record
	if err := q.Get(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func LatestSemver(ctx context.Context, db *sql.DB, dialect ormdriver.Dialect, prefix, tenant string) (string, error) {
	table := prefix + "registry_snapshots"
	q := query.New(db, table, dialect).
		Select("semver").
		Where("tenant_id", tenant).
		OrderBy("id", "desc").
		Limit(1).
		WithContext(ctx)

	var row struct{ Semver string }
	if err := q.First(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "0.0.0", nil
		}
		return "", err
	}
	return row.Semver, nil
}

func NextSemver(prev, bump string) string {
	var maj, min, patch int
	fmt.Sscanf(prev, "%d.%d.%d", &maj, &min, &patch)
	switch bump {
	case "major":
		maj++
		min, patch = 0, 0
	case "minor":
		min++
		patch = 0
	default:
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", maj, min, patch)
}

// Encode compresses the given []byte data.
func Encode(b []byte) ([]byte, error) {
	return compress(b)
}

// EncodeAny marshals the given data to YAML if necessary and compresses it.
func EncodeAny(v any) ([]byte, error) {
	var b []byte
	switch t := v.(type) {
	case []byte:
		b = t
	default:
		var err error
		b, err = yaml.Marshal(v)
		if err != nil {
			return nil, err
		}
	}
	return compress(b)
}
func Decode(data []byte) ([]byte, error) { return decompress(data) }

func ParsePatch(ver string) int {
	var maj, min, patch int
	fmt.Sscanf(ver, "%d.%d.%d", &maj, &min, &patch)
	return patch
}

func NextPatch(prev string) string {
	n := ParsePatch(prev)
	return "0.0." + strconv.Itoa(n+1)
}

// parseDBTime attempts to convert a database time value to time.Time.
// Supported input types are: time.Time, []byte, and string.
// Returns an error if the input type is not supported or cannot be parsed.
func parseDBTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case []byte:
		return parseTimeString(string(t))
	case string:
		return parseTimeString(t)
	default:
		return time.Time{}, fmt.Errorf("unsupported time type: %T", v)
	}
}

func parseTimeString(s string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", time.RFC3339}
	for _, l := range layouts {
		if ts, err := time.Parse(l, s); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q using any of the supported layouts: [%s]", s, strings.Join(layouts, ", "))
}
