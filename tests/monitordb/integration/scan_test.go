//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/pkg/crypto"
)

func TestAddAndScan(t *testing.T) {
	os.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")
	ctx := context.Background()
	central, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("core"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	if err != nil {
		t.Skipf("central container: %v", err)
	}
	defer central.Terminate(ctx)
	remote, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("target"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	if err != nil {
		t.Skipf("remote container: %v", err)
	}
	defer remote.Terminate(ctx)
	centralDSN, _ := central.ConnectionString(ctx)
	remoteDSN, _ := remote.ConnectionString(ctx)
	centralDB, err := sql.Open("postgres", centralDSN)
	if err != nil {
		t.Fatalf("central open: %v", err)
	}
	defer centralDB.Close()
	remoteDB, err := sql.Open("postgres", remoteDSN)
	if err != nil {
		t.Fatalf("remote open: %v", err)
	}
	defer remoteDB.Close()

	mig := migrator.NewWithDriver("postgres")
	if err := mig.Up(ctx, centralDB, 13); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := remoteDB.ExecContext(ctx, `CREATE TABLE posts(id SERIAL PRIMARY KEY, title TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	repo := &monitordb.Repo{DB: centralDB, Driver: "postgres"}
	enc, err := crypto.Encrypt([]byte(remoteDSN))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	id, err := repo.Create(ctx, monitordb.Database{TenantID: "t1", Name: "remote", Driver: "postgres", DSNEnc: enc})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	if _, _, _, _, err := monitordb.ScanDatabase(ctx, repo, id, "t1"); err != nil {
		t.Fatalf("scan: %v", err)
	}
	var cnt int
	if err := centralDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM gcfm_custom_fields WHERE tenant_id='t1' AND db_id=$1`, id).Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt == 0 {
		t.Fatalf("expected custom fields")
	}
}

func TestScanErrorsAndSkip(t *testing.T) {
	os.Setenv("CF_ENC_KEY", "0123456789abcdef0123456789abcdef")
	ctx := context.Background()
	central, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("core"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	if err != nil {
		t.Skipf("central: %v", err)
	}
	defer central.Terminate(ctx)
	remote, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("target"), postgres.WithUsername("user"), postgres.WithPassword("pass"))
	if err != nil {
		t.Skipf("remote: %v", err)
	}
	defer remote.Terminate(ctx)
	centralDSN, _ := central.ConnectionString(ctx)
	centralDB, _ := sql.Open("postgres", centralDSN)
	defer centralDB.Close()
	mig := migrator.NewWithDriver("postgres")
	if err := mig.Up(ctx, centralDB, 13); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := &monitordb.Repo{DB: centralDB, Driver: "postgres"}
	// permission error
	remoteDSN, _ := remote.ConnectionString(ctx)
	wrongDSN := strings.Replace(remoteDSN, "pass", "bad", 1)
	encWrong, _ := crypto.Encrypt([]byte(wrongDSN))
	id, err := repo.Create(ctx, monitordb.Database{TenantID: "t1", Name: "bad", Driver: "postgres", DSNEnc: encWrong})
	if err != nil {
		t.Fatalf("create bad: %v", err)
	}
	if _, _, _, _, err := monitordb.ScanDatabase(ctx, repo, id, "t1"); err == nil {
		t.Fatalf("expected error for bad credentials")
	}
	// reserved table skip
	remoteDB, _ := sql.Open("postgres", remoteDSN)
	defer remoteDB.Close()
	if _, err := remoteDB.ExecContext(ctx, `CREATE TABLE gcfm_users(id SERIAL PRIMARY KEY)`); err != nil {
		t.Fatalf("create reserved: %v", err)
	}
	enc, _ := crypto.Encrypt([]byte(remoteDSN))
	rid, err := repo.Create(ctx, monitordb.Database{TenantID: "t1", Name: "res", Driver: "postgres", DSNEnc: enc})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(file), "..", "..", "..")
	reserved.Load(filepath.Join(base, "configs", "default.yaml"))
	_, ins, _, skipped, err := monitordb.ScanDatabase(ctx, repo, rid, "t1")
	if err != nil {
		t.Fatalf("scan reserved: %v", err)
	}
	if ins != 0 || len(skipped) == 0 {
		t.Fatalf("expected skip for reserved table")
	}
}
