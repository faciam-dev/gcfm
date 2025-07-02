package metrics_test

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/faciam-dev/gcfm/internal/server"
)

func TestMetricsEndpoint(t *testing.T) {
	os.Setenv("JWT_SECRET", "test")
	api := server.New(nil, "", "")
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	api.Adapter().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "cf_cache_hits_total") {
		t.Fatalf("metric missing")
	}
}
