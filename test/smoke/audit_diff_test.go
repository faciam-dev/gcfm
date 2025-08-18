package smoke

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faciam-dev/gcfm/pkg/crypto"
)

func TestAuditDiffAndCounts(t *testing.T) {
	e := newEnv(t)
	defer e.close()

	enc, err := crypto.Encrypt([]byte(os.Getenv("TEST_DATABASE_URL")))
	if err != nil {
		t.Fatalf("encrypt dsn: %v", err)
	}
	mustExec(t, e.DB, `
        INSERT INTO gcfm_monitored_databases(id, tenant_id, name, driver, dsn_enc, created_at)
        VALUES (1,'t1','local','postgres',$1,NOW())
        ON CONFLICT (id) DO UPDATE SET tenant_id='t1', name='local', driver='postgres', dsn_enc=$1
        `, enc)

	jwt := signJWT(t, e.Secret, "1", "t1", "admin", time.Hour)

	body := `{"db_id":1,"table":"posts","column":"diff_test_col","type":"text"}`
	reqCreate, _ := http.NewRequest("POST", e.URL+"/v1/custom-fields", strings.NewReader(body))
	reqCreate.Header.Set("Authorization", "Bearer "+jwt)
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("X-Tenant-ID", "t1")
	_ = doJSON(t, reqCreate)

	req, _ := http.NewRequest("GET", e.URL+"/v1/audit-logs?page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Tenant-ID", "t1")
	list := doJSON(t, req)
	if jlen(list, "items") == 0 {
		t.Fatal("no audit logs")
	}

	var id, cc int
	for i := 0; i < jlen(list, "items"); i++ {
		if c := jint(list, fmt.Sprintf("items.%d.changeCount", i)); c > 0 {
			id = jint(list, fmt.Sprintf("items.%d.id", i))
			cc = c
			break
		}
	}
	if id == 0 {
		t.Fatal("no diffable audit log found")
	}

	req2, _ := http.NewRequest("GET", e.URL+fmt.Sprintf("/v1/audit-logs/%d/diff", id), nil)
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

	req3, _ := http.NewRequest("GET", e.URL+"/v1/audit-logs?min_changes=0&max_changes=0", nil)
	req3.Header.Set("Authorization", "Bearer "+jwt)
	req3.Header.Set("X-Tenant-ID", "t1")
	zero := doJSON(t, req3)
	if n := jlen(zero, "items"); n != 0 {
		t.Fatalf("expected zero items, got %d", n)
	}
}
