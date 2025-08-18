package smoke

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/server"
)

type Env struct {
	DB     *sql.DB
	URL    string
	HTTP   *httptest.Server
	Now    time.Time
	Secret []byte
}

func newEnv(t *testing.T) *Env {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	os.Setenv("CF_ENC_KEY", strings.Repeat("0", 32))
	secret := []byte("test-secret")
	os.Setenv("JWT_SECRET", string(secret))

	runMigrations(t, dsn)
	seed(t, db)

	mux := buildRouterForTest(t, db)
	ts := httptest.NewServer(mux)

	return &Env{DB: db, URL: ts.URL, HTTP: ts, Now: time.Now().UTC(), Secret: secret}
}

func (e *Env) close() {
	e.HTTP.Close()
	e.DB.Close()
}

func buildRouterForTest(t *testing.T, db *sql.DB) http.Handler {
	t.Helper()
	cfg := server.DBConfig{Driver: "postgres", DSN: os.Getenv("TEST_DATABASE_URL"), TablePrefix: "gcfm_"}
	api := server.New(db, cfg)
	return api.Adapter()
}

func runMigrations(t *testing.T, dsn string) {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db for migrate: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	cmd := exec.Command("go", "run", "../../cmd/fieldctl", "db", "migrate", "--db", dsn, "--schema", "public", "--driver", "postgres", "--table-prefix", "gcfm_", "--seed")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
}

func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	// Clean existing data
	mustExec(t, db, `DELETE FROM gcfm_user_roles; DELETE FROM gcfm_roles; DELETE FROM gcfm_role_policies; DELETE FROM gcfm_users; DELETE FROM gcfm_audit_logs;`)
	mustExec(t, db, `ALTER SEQUENCE gcfm_users_id_seq RESTART WITH 1; ALTER SEQUENCE gcfm_roles_id_seq RESTART WITH 1; ALTER SEQUENCE gcfm_audit_logs_id_seq RESTART WITH 1;`)

	// Users: t1 has 30, t2 has 3
	for i := 1; i <= 30; i++ {
		mustExec(t, db, `INSERT INTO gcfm_users(tenant_id, username, password_hash, created_at) VALUES('t1', $1, 'x', NOW() - ($2||' minutes')::interval)`, uname(i), i)
	}
	for i := 1; i <= 3; i++ {
		mustExec(t, db, `INSERT INTO gcfm_users(tenant_id, username, password_hash, created_at) VALUES('t2', $1, 'x', NOW())`, "x"+uname(i))
	}

	// Roles and policies
	mustExec(t, db, `INSERT INTO gcfm_roles(name) VALUES('admin') ON CONFLICT (name) DO NOTHING`)
	var roleID int
	if err := db.QueryRow(`SELECT id FROM gcfm_roles WHERE name='admin'`).Scan(&roleID); err != nil {
		t.Fatalf("get role id: %v", err)
	}
	for _, m := range []string{"GET", "POST", "PUT", "DELETE"} {
		mustExec(t, db, `INSERT INTO gcfm_role_policies(role_id, path, method) VALUES($1,'/v1/*',$2) ON CONFLICT DO NOTHING`, roleID, m)
	}
	// Assign user 1 as admin
	mustExec(t, db, `INSERT INTO gcfm_user_roles(user_id, role_id) VALUES(1, $1) ON CONFLICT DO NOTHING`, roleID)

	// Audit logs
	mustExec(t, db, `
    INSERT INTO gcfm_audit_logs(tenant_id, actor, action, table_name, column_name, before_json, after_json, added_count, removed_count, change_count, applied_at)
    VALUES ('t1','1','update','acl','new1', $1::jsonb, $2::jsonb, 1,0,1, NOW());
  `, "{}", `{"a":1}`)
}

func uname(i int) string { return "user" + fmt.Sprintf("%02d", i) }

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %s: %v", q, err)
	}
}

func signJWT(t *testing.T, secret []byte, sub, tenant, role string, ttl time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   sub,
		"tid":   tenant,
		"roles": []string{role},
		"exp":   time.Now().Add(ttl).Unix(),
		"iat":   time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return s
}

func doJSON(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m
}

func jlen(m map[string]any, path string) int {
	v := jpath(m, path)
	if a, ok := v.([]any); ok {
		return len(a)
	}
	return 0
}

func jint(m map[string]any, path string) int {
	v := jpath(m, path)
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

func jget(m map[string]any, path string) string {
	v := jpath(m, path)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func jpath(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		switch c := cur.(type) {
		case map[string]any:
			cur = c[p]
		case []any:
			idx, _ := strconv.Atoi(p)
			if idx < 0 || idx >= len(c) {
				return nil
			}
			cur = c[idx]
		default:
			return nil
		}
	}
	return cur
}
