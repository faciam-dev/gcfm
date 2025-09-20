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
	"path/filepath"
	"time"

	_ "github.com/lib/pq"

	"github.com/faciam-dev/gcfm/internal/config"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/monitordb"
	"github.com/faciam-dev/gcfm/internal/server"
	"github.com/faciam-dev/gcfm/pkg/crypto"
	md "github.com/faciam-dev/gcfm/pkg/metadata"
	"github.com/faciam-dev/gcfm/pkg/util"
	"github.com/go-co-op/gocron"
)

func main() {
	dsn := flag.String("dsn", "", "database DSN")
	driver := flag.String("driver", "postgres", "database driver")
	tblPrefix := flag.String("table-prefix", util.GetEnv("TABLE_PREFIX", "gcfm_"), "registry table prefix (default gcfm_)")
	addr := flag.String("addr", ":8080", "listen address")
	openapi := flag.String("openapi", "", "write OpenAPI JSON and exit")
	flag.Parse()

	logger.Set(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	driverProvided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "driver" {
			driverProvided = true
		}
	})

	if *dsn != "" {
		if detected, err := util.DetectDriver(*dsn); err != nil {
			if !driverProvided || *driver == "" {
				logger.L.Error("detect driver", "dsn", *dsn, "err", err)
				os.Exit(1)
			}
		} else {
			if !driverProvided || *driver == "" {
				*driver = detected
			} else if detected != "" && *driver != detected {
				logger.L.Error("driver mismatch", "driver", *driver, "dsn", *dsn, "expected", detected)
				os.Exit(1)
			}
		}
	}

	if err := crypto.CheckEnv(); err != nil {
		logger.L.Error("crypto key", "err", err)
		os.Exit(1)
	}

	var db *sql.DB
	var err error
	dialect := util.DialectFromDriver(*driver)
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

	dbCfg := server.DBConfig{Driver: *driver, DSN: *dsn, TablePrefix: cfg.TablePrefix}
	log.Printf("table prefix: %q", dbCfg.TablePrefix)

	api := server.New(db, dbCfg)

	if db != nil {
		repo := &monitordb.Repo{DB: db, Driver: dbCfg.Driver, Dialect: dialect, TablePrefix: dbCfg.TablePrefix}
		s := gocron.NewScheduler(time.UTC)
		if _, err := s.Cron("0 3 * * *").Do(func() {
			ctx := context.Background()
			dbs, err := repo.ListAll(ctx)
			if err != nil {
				logger.L.Error("list databases", "err", err)
				return
			}
			if err := monitordb.ScanAll(ctx, repo, dbs); err != nil {
				logger.L.Error("scan all databases", "err", err)
			}
		}); err != nil {
			logger.L.Error("schedule db scan", "err", err)
		}
		s.StartAsync()
	}

	if *openapi != "" {
		data, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
		if err != nil {
			logger.L.Error("marshal openapi", "err", err)
			os.Exit(1)
		}
		p := filepath.Clean(*openapi)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			logger.L.Error("write openapi", "err", err)
			os.Exit(1)
		}
		return
	}

	logger.L.Info("listening", "addr", *addr)
	srv := &http.Server{
		Addr:         *addr,
		Handler:      api.Adapter(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.L.Error("server error", "err", err)
		os.Exit(1)
	}
}
