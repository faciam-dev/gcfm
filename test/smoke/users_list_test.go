package smoke

import (
	"net/http"
	"testing"
	"time"
)

func TestUsersPagingAndSort(t *testing.T) {
	e := newEnv(t)
	defer e.close()

	jwt := signJWT(e.Secret, "1", "t1", "admin", time.Hour)

	req, _ := http.NewRequest("GET", e.URL+"/v1/rbac/users?sort=username&order=desc&page=2&per_page=10", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Tenant-ID", "t1")
	body := doJSON(t, req)

	if jlen(body, "items") != 10 {
		t.Fatalf("want 10 items")
	}
	first := jget(body, "items.0.username")
	last := jget(body, "items.9.username")
	if first != "user20" || last != "user11" {
		t.Fatalf("unexpected page2 ordering: first=%v last=%v", first, last)
	}
}
