package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/config"
	cfregistry "github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	md "github.com/faciam-dev/gcfm/pkg/metadata"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/go-co-op/gocron"
)

func main() {
	dsn := flag.String("dsn", "", "database DSN")
	driver := flag.String("driver", "postgres", "database driver")
	tblPrefix := flag.String("table-prefix", getenv("TABLE_PREFIX", "gcfm_"), "registry table prefix (default gcfm_)")
	addr := flag.String("addr", ":8080", "listen address")
	openapi := flag.String("openapi", "", "write OpenAPI JSON and exit")
	flag.Parse()

	logger.Set(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	if err := crypto.CheckEnv(); err != nil {
		logger.L.Error("crypto key", "err", err)
		os.Exit(1)
	}

	var db *sql.DB
	var err error
	var dialect ormdriver.Dialect
	if *driver == "postgres" {
		dialect = ormdriver.PostgresDialect{}
	} else {
		dialect = ormdriver.MySQLDialect{}
	}
	if *dsn != "" {
		db, err = sql.Open(*driver, *dsn)
		if err != nil {
			logger.L.Error("db open", "err", err)
			os.Exit(1)
		}
		if err := config.CheckPrefix(context.Background(), db, dialect, *tblPrefix); err != nil {
			logger.L.Error("prefix check", "err", err)
			os.Exit(1)
		}
		defer db.Close()
	}

	cfg := config.Config{TablePrefix: *tblPrefix}
	md.SetTablePrefix(cfg.TablePrefix)
	cfregistry.SetTablePrefix(cfg.TablePrefix)

	dbCfg := server.DBConfig{Driver: *driver, DSN: *dsn, TablePrefix: cfg.TablePrefix}
	log.Printf("table prefix: %q", dbCfg.TablePrefix)

	api := server.New(db, dbCfg)

	if db != nil {
		repo := &monitordb.Repo{DB: db, Driver: dbCfg.Driver, Dialect: dialect, TablePrefix: dbCfg.TablePrefix}
		s := gocron.NewScheduler(time.UTC)
		s.Cron("0 3 * * *").Do(func() {
			ctx := context.Background()
			dbs, err := repo.ListAll(ctx)
			if err == nil {
				monitordb.ScanAll(ctx, repo, dbs)
			}
		})
		s.StartAsync()
	}

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

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
