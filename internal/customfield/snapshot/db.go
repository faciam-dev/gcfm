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
	"time"
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
		_ = db.QueryRowContext(ctx, "SELECT taken_at FROM gcfm_registry_snapshots WHERE id=?", id).Scan(&res.TakenAt)
	}
	return res, nil
}

func Get(ctx context.Context, db *sql.DB, driver, tenant, ver string) (Record, error) {
	q := "SELECT id, semver, yaml, taken_at, author FROM gcfm_registry_snapshots WHERE tenant_id=$1 AND semver=$2"
	if driver != "postgres" {
		q = "SELECT id, semver, yaml, taken_at, author FROM gcfm_registry_snapshots WHERE tenant_id=? AND semver=?"
	}
	var r Record
	err := db.QueryRowContext(ctx, q, tenant, ver).Scan(&r.ID, &r.Semver, &r.YAML, &r.TakenAt, &r.Author)
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
		if err := rows.Scan(&r.ID, &r.Semver, &r.TakenAt, &r.Author); err != nil {
			return nil, err
		}
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

func Encode(data []byte) ([]byte, error) { return compress(data) }
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
