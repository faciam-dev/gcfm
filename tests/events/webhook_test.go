package events_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/faciam-dev/gcfm/internal/events"
)

func TestWebhookSignature(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		if len(body) == 0 {
			t.Errorf("no body")
		}
		gotSig = r.Header.Get("X-CF-Signature")
	}))
	defer srv.Close()
	wh := events.NewWebhookSink(events.WebhookConfig{Enabled: true, Endpoint: srv.URL, Secret: "s"})
	evt := events.Event{Name: "n"}
	if err := wh.Emit(context.Background(), evt); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if gotSig == "" {
		t.Fatalf("missing signature")
	}
}
