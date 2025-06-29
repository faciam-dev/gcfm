package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/server"
)

func main() {
	dsn := flag.String("dsn", "", "Postgres DSN")
	addr := flag.String("addr", ":8080", "listen address")
	openapi := flag.String("openapi", "", "write OpenAPI JSON and exit")
	flag.Parse()

	var db *sql.DB
	var err error
	if *dsn != "" {
		db, err = sql.Open("postgres", *dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
	}

	api := server.New(db)

	if *openapi != "" {
		data, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*openapi, data, 0644); err != nil {
			log.Fatal(err)
		}
		return
	}

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, api.Adapter()))
}
