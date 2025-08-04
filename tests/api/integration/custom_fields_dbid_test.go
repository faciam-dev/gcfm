//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/faciam-dev/gcfm/internal/customfield/migrator"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	sdk "github.com/faciam-dev/gcfm/sdk"
)

func TestAPI_ListCustomFieldsByDBID(t *testing.T) {
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
	remoteDSN, _ := remote.ConnectionString(ctx)
	centralDB, _ := sql.Open("postgres", centralDSN)
	defer centralDB.Close()
	remoteDB, _ := sql.Open("postgres", remoteDSN)
	defer remoteDB.Close()

	mig := migrator.NewWithDriver("postgres")
	if err := mig.Up(ctx, centralDB, 13); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := remoteDB.ExecContext(ctx, `CREATE TABLE posts(id SERIAL PRIMARY KEY, title TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	repo := &monitordb.Repo{DB: centralDB, Driver: "postgres"}
	enc, _ := crypto.Encrypt([]byte(remoteDSN))
	id, err := repo.Create(ctx, monitordb.Database{TenantID: "t1", Name: "remote", Driver: "postgres", DSNEnc: enc})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, _, _, err := monitordb.ScanDatabase(ctx, repo, id, "t1"); err != nil {
		t.Fatalf("scan: %v", err)
	}

	t.Setenv("JWT_SECRET", "testsecret")
	api := server.New(centralDB, "postgres", centralDSN)
	srv := httptest.NewServer(api.Adapter())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/custom-fields?db_id=%d", srv.URL, id), nil)
	req.Header.Set("X-Tenant-ID", "t1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var metas []sdk.FieldMeta
	if err := json.NewDecoder(resp.Body).Decode(&metas); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(metas) == 0 {
		t.Fatalf("expected fields")
	}
}
