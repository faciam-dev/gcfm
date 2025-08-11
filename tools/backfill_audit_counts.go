package main

import (
	"context"
	"database/sql"
	"flag"
	"log"

	auditutil "github.com/faciam-dev/gcfm/pkg/audit"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func main() {
	driver := flag.String("driver", "postgres", "database driver (postgres or mysql)")
	dsn := flag.String("dsn", "", "database DSN")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("dsn is required")
	}

	db, err := sql.Open(*driver, *dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	sel := "SELECT id, COALESCE(before_json::text,'{}'), COALESCE(after_json::text,'{}') FROM gcfm_audit_logs WHERE change_count = 0"
	upd := "UPDATE gcfm_audit_logs SET added_count=$1, removed_count=$2, change_count=$3 WHERE id=$4"
	if *driver == "mysql" {
		sel = "SELECT id, COALESCE(JSON_UNQUOTE(before_json), '{}'), COALESCE(JSON_UNQUOTE(after_json), '{}') FROM gcfm_audit_logs WHERE change_count = 0"
		upd = "UPDATE gcfm_audit_logs SET added_count=?, removed_count=?, change_count=? WHERE id=?"
	}

	rows, err := db.QueryContext(ctx, sel)
	if err != nil {
		log.Fatalf("select: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var before, after string
		if err := rows.Scan(&id, &before, &after); err != nil {
			log.Fatalf("scan: %v", err)
		}
		_, add, del := auditutil.UnifiedDiff([]byte(before), []byte(after))
		if _, err := db.ExecContext(ctx, upd, add, del, add+del, id); err != nil {
			log.Fatalf("update %d: %v", id, err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("rows: %v", err)
	}
}
