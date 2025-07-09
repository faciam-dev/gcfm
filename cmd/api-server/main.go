package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/server"
)

func main() {
	dsn := flag.String("dsn", "", "database DSN")
	driver := flag.String("driver", "postgres", "database driver")
	addr := flag.String("addr", ":8080", "listen address")
	openapi := flag.String("openapi", "", "write OpenAPI JSON and exit")
	flag.Parse()

	logger.Set(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	var db *sql.DB
	var err error
	if *dsn != "" {
		db, err = sql.Open(*driver, *dsn)
		if err != nil {
			logger.L.Error("db open", "err", err)
			os.Exit(1)
		}
		defer db.Close()
	}

	api := server.New(db, *driver, *dsn)

	if *openapi != "" {
		data, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
		if err != nil {
			logger.L.Error("marshal openapi", "err", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*openapi, data, 0644); err != nil {
			logger.L.Error("write openapi", "err", err)
			os.Exit(1)
		}
		return
	}

	logger.L.Info("listening", "addr", *addr)
	if err := http.ListenAndServe(*addr, api.Adapter()); err != nil {
		logger.L.Error("server error", "err", err)
		os.Exit(1)
	}
}
