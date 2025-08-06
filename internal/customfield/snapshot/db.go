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

func Insert(ctx context.Context, db *sql.DB, driver, tenant, semver, author string, yaml []byte) (Record, error) {
	q := "INSERT INTO gcfm_registry_snapshots(tenant_id, semver, yaml, author) VALUES ($1,$2,$3,$4) RETURNING id, taken_at"
	if driver != "postgres" {
		q = "INSERT INTO gcfm_registry_snapshots(tenant_id, semver, yaml, author) VALUES (?,?,?,?)"
	}
	res := Record{Semver: semver, Author: author}
	if driver == "postgres" {
		if err := db.QueryRowContext(ctx, q, tenant, semver, yaml, author).Scan(&res.ID, &res.TakenAt); err != nil {
			return Record{}, err
		}
	} else {
		r, err := db.ExecContext(ctx, q, tenant, semver, yaml, author)
		if err != nil {
			return Record{}, err
		}
		id, _ := r.LastInsertId()
		res.ID = id
		var t any
		if err := db.QueryRowContext(ctx, "SELECT taken_at FROM gcfm_registry_snapshots WHERE id=?", id).Scan(&t); err == nil {
			if ts, err := parseDBTime(t); err == nil {
				res.TakenAt = ts
			} else {
				return Record{}, err
			}
		} else {
			return Record{}, err
		}
	}
	return res, nil
}

func Get(ctx context.Context, db *sql.DB, driver, tenant, ver string) (Record, error) {
	q := "SELECT id, semver, yaml, taken_at, author FROM gcfm_registry_snapshots WHERE tenant_id=$1 AND semver=$2"
	if driver != "postgres" {
		q = "SELECT id, semver, yaml, taken_at, author FROM gcfm_registry_snapshots WHERE tenant_id=? AND semver=?"
	}
	var r Record
	var t any
	err := db.QueryRowContext(ctx, q, tenant, ver).Scan(&r.ID, &r.Semver, &r.YAML, &t, &r.Author)
	if err != nil {
		return r, err
	}
	r.TakenAt, err = parseDBTime(t)
	return r, err
}

func List(ctx context.Context, db *sql.DB, driver, tenant string, limit int) ([]Record, error) {
	if limit == 0 {
		limit = 20
	}
	q := "SELECT id, semver, taken_at, author FROM gcfm_registry_snapshots WHERE tenant_id=$1 ORDER BY id DESC LIMIT $2"
	if driver != "postgres" {
		q = "SELECT id, semver, taken_at, author FROM gcfm_registry_snapshots WHERE tenant_id=? ORDER BY id DESC LIMIT ?"
	}
	rows, err := db.QueryContext(ctx, q, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var t any
		if err := rows.Scan(&r.ID, &r.Semver, &t, &r.Author); err != nil {
			return nil, err
		}
		ts, err := parseDBTime(t)
		if err != nil {
			return nil, err
		}
		r.TakenAt = ts
		out = append(out, r)
	}
	return out, rows.Err()
}

func LatestSemver(ctx context.Context, db *sql.DB, driver, tenant string) (string, error) {
	q := "SELECT semver FROM gcfm_registry_snapshots WHERE tenant_id=$1 ORDER BY id DESC LIMIT 1"
	if driver != "postgres" {
		q = "SELECT semver FROM gcfm_registry_snapshots WHERE tenant_id=? ORDER BY id DESC LIMIT 1"
	}
	var s string
	err := db.QueryRowContext(ctx, q, tenant).Scan(&s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "0.0.0", nil
		}
		return "", err
	}
	return s, nil
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
