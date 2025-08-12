package smoke

import (
	"net/http"
	"testing"
	"time"
)

func TestTenantIsolation(t *testing.T) {
	e := newEnv(t)
	defer e.close()

	jwt1 := signJWT(t, e.Secret, "1", "t1", "admin", time.Hour)
	req1, _ := http.NewRequest("GET", e.URL+"/v1/rbac/users?page=1&per_page=100", nil)
	req1.Header.Set("Authorization", "Bearer "+jwt1)
	req1.Header.Set("X-Tenant-ID", "t1")
	resp1 := doJSON(t, req1)
	got1 := jlen(resp1, "items")
	if got1 != 30 {
		t.Fatalf("t1 should see 30 users, got %d", got1)
	}

	jwt2 := signJWT(t, e.Secret, "1", "t2", "admin", time.Hour)
	req2, _ := http.NewRequest("GET", e.URL+"/v1/rbac/users?page=1&per_page=100", nil)
	req2.Header.Set("Authorization", "Bearer "+jwt2)
	req2.Header.Set("X-Tenant-ID", "t2")
	resp2 := doJSON(t, req2)
	got2 := jlen(resp2, "items")
	if got2 != 3 {
		t.Fatalf("t2 should see 3 users, got %d", got2)
	}
}
