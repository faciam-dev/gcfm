package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdk "github.com/faciam-dev/gcfm/sdk"
	client "github.com/faciam-dev/gcfm/sdk/client"
)

type record struct{ create, update, delete bool }

func TestHTTPClient(t *testing.T) {
	rec := &record{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/custom-fields", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]sdk.FieldMeta{{TableName: "t", ColumnName: "c"}})
		case http.MethodPost:
			rec.create = true
			w.WriteHeader(http.StatusCreated)
		}
	})
	mux.HandleFunc("/v1/custom-fields/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/v1/custom-fields/"):]
		if id == "t.c" && r.Method == http.MethodPut {
			rec.update = true
		}
		if id == "t.c" && r.Method == http.MethodDelete {
			rec.delete = true
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := client.NewHTTP(srv.URL)
	if c.Mode() != "http" {
		t.Fatalf("mode %s", c.Mode())
	}
	if _, err := c.List(context.Background(), ""); err != nil {
		t.Fatalf("list %v", err)
	}
	if err := c.Create(context.Background(), sdk.FieldMeta{TableName: "t", ColumnName: "c"}); err != nil {
		t.Fatalf("create %v", err)
	}
	if err := c.Update(context.Background(), sdk.FieldMeta{TableName: "t", ColumnName: "c"}); err != nil {
		t.Fatalf("update %v", err)
	}
	if err := c.Delete(context.Background(), "t", "c"); err != nil {
		t.Fatalf("delete %v", err)
	}
	if !rec.create || !rec.update || !rec.delete {
		t.Fatalf("handlers not hit: %#v", rec)
	}
}
