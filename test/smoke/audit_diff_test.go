package smoke

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAuditDiffAndCounts(t *testing.T) {
	e := newEnv(t)
	defer e.close(t)

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set")
	}

	jwt := signJWT(t, e.Secret, "1", "t1", "admin", time.Hour)

	dbBody := fmt.Sprintf(`{"name":"local","driver":"postgres","dsn":"%s"}`, dsn)
	reqDB, err := http.NewRequest("POST", e.URL+"/v1/databases", strings.NewReader(dbBody))
	if err != nil {
		t.Fatal(err)
	}
	reqDB.Header.Set("Authorization", "Bearer "+jwt)
	reqDB.Header.Set("Content-Type", "application/json")
	reqDB.Header.Set("X-Tenant-ID", "t1")
	dbRes := doJSON(t, reqDB)
	dbID := jint(dbRes, "id")

	since := time.Now().UTC().Format(time.RFC3339)

	body := fmt.Sprintf(`{"db_id":%d,"table":"posts","column":"diff_test_col","type":"text","display":{"widget":"text"}}`, dbID)
	reqCreate, err := http.NewRequest("POST", e.URL+"/v1/custom-fields", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	reqCreate.Header.Set("Authorization", "Bearer "+jwt)
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("X-Tenant-ID", "t1")
	_ = doJSON(t, reqCreate)

	req, err := http.NewRequest("GET", e.URL+fmt.Sprintf("/v1/audit-logs?limit=10&db_id=%d&table=posts&from=%s", dbID, url.QueryEscape(since)), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Tenant-ID", "t1")
	list := doJSON(t, req)
	if jlen(list, "items") == 0 {
		t.Fatal("no audit logs")
	}

	var id, cc int
	for i := 0; i < jlen(list, "items"); i++ {
		if jget(list, fmt.Sprintf("items.%d.tableName", i)) != "posts" {
			continue
		}
		if c := jint(list, fmt.Sprintf("items.%d.changeCount", i)); c > 0 {
			id = jint(list, fmt.Sprintf("items.%d.id", i))
			cc = c
			break
		}
	}
	if id == 0 {
		t.Fatalf("no diffable audit log found for db_id=%d table=posts", dbID)
	}

	req2, err := http.NewRequest("GET", e.URL+fmt.Sprintf("/v1/audit-logs/%d/diff", id), nil)
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Authorization", "Bearer "+jwt)
	req2.Header.Set("X-Tenant-ID", "t1")
	diff := doJSON(t, req2)

	uni := jget(diff, "unified")
	add := jint(diff, "added")
	del := jint(diff, "removed")

	if len(uni) == 0 {
		t.Fatal("unified diff should not be empty")
	}
	if add+del != cc {
		t.Fatalf("count mismatch: list=%d diff(add+del)=%d", cc, add+del)
	}

	req3, err := http.NewRequest("GET", e.URL+"/v1/audit-logs?min_changes=0&max_changes=0", nil)
	if err != nil {
		t.Fatal(err)
	}
	req3.Header.Set("Authorization", "Bearer "+jwt)
	req3.Header.Set("X-Tenant-ID", "t1")
	zero := doJSON(t, req3)
	if n := jlen(zero, "items"); n != 0 {
		t.Fatalf("expected zero items, got %d", n)
	}
}
