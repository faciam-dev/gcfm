package smoke

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAuditDiffAndCounts(t *testing.T) {
	e := newEnv(t)
	defer e.close()

	jwt := signJWT(e.Secret, "1", "t1", "admin", time.Hour)

	req, _ := http.NewRequest("GET", e.URL+"/v1/audit-logs?page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Tenant-ID", "t1")
	list := doJSON(t, req)
	if jlen(list, "items") == 0 {
		t.Fatal("no audit logs")
	}

	id := jint(list, "items.0.id")
	cc := jint(list, "items.0.changeCount")

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
	_ = jlen(zero, "items")
}
