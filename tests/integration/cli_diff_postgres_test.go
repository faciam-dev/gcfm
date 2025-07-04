//go:build integration
// +build integration

package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var fieldctlBin string

func TestMain(m *testing.M) {
	if out, err := exec.Command("go", "build", "-o", "fieldctl", "../../cmd/fieldctl").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build fieldctl: %v\n%s", err, out)
		os.Exit(1)
	}
	fieldctlBin = "./fieldctl"
	code := m.Run()
	os.Remove(fieldctlBin)
	os.Exit(code)
}

func runCmd(cmd *exec.Cmd) ([]byte, error) { return cmd.CombinedOutput() }

func buildFieldctlCommand(args ...string) *exec.Cmd {
	base := append([]string{fieldctlBin}, args...)
	return exec.Command(base[0], base[1:]...)
}

func setupPG(t *testing.T) (context.Context, string, *sql.DB) {
	ctx := context.Background()
	container, err := func() (c *postgres.PostgresContainer, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		return postgres.Run(ctx, "postgres:16", postgres.WithDatabase("app"), postgres.WithUsername("postgres"), postgres.WithPassword("pass"))
	}()
	if err != nil {
		t.Skipf("container: %v", err)
	}
	if container == nil {
		t.Fatalf("container nil")
	}
	t.Cleanup(func() { container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return ctx, dsn, db
}

func TestCLIDiff_NoChange(t *testing.T) {
	_, dsn, _ := setupPG(t)
	file := filepath.Join("tests", "testdata", "generator", "registry.yaml")
	if out, err := runCmd(buildFieldctlCommand("db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres")); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
	if out, err := runCmd(buildFieldctlCommand("apply", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", file)); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	cmd := buildFieldctlCommand("diff", "--db", dsn, "--schema", "public", "--file", file, "--fail-on-change")
	if out, err := runCmd(cmd); err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
}

func TestCLIDiff_Change(t *testing.T) {
	ctx, dsn, db := setupPG(t)
	file := filepath.Join("tests", "testdata", "generator", "registry.yaml")
	if out, err := runCmd(buildFieldctlCommand("db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres")); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
	if out, err := runCmd(buildFieldctlCommand("apply", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", file)); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO gcfm_custom_fields(table_name,column_name,data_type) VALUES ('posts','extra','text')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	cmd := buildFieldctlCommand("diff", "--db", dsn, "--schema", "public", "--file", file, "--fail-on-change")
	if out, err := runCmd(cmd); err == nil {
		t.Fatalf("expected exit 2")
	} else if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 2 {
		t.Fatalf("code=%d\n%s", ee.ExitCode(), out)
	}
}

func TestCLIDiff_MarkdownFormat(t *testing.T) {
	ctx, dsn, db := setupPG(t)
	file := filepath.Join("tests", "testdata", "generator", "registry.yaml")
	if out, err := runCmd(buildFieldctlCommand("db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres")); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
	if out, err := runCmd(buildFieldctlCommand("apply", "--db", dsn, "--schema", "public", "--driver", "postgres", "--file", file)); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO gcfm_custom_fields(table_name,column_name,data_type) VALUES ('posts','extra','text')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	out, err := runCmd(buildFieldctlCommand("diff", "--db", dsn, "--schema", "public", "--file", file, "--format", "markdown", "--fail-on-change"))
	if err == nil {
		t.Fatalf("expected non-zero exit")
	}
	if !strings.HasPrefix(string(out), "- `") {
		t.Fatalf("unexpected output: %s", out)
	}
}
