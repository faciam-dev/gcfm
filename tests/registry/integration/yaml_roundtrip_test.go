package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
	mysqlscanner "github.com/faciam-dev/gcfm/internal/driver/mysql"
)

func TestExportApplyRoundTrip(t *testing.T) {
	ctx := context.Background()
	t.Skip("integration test requires Docker")
	var err error
	container, err := func() (*mysql.MySQLContainer, error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		return mysql.Run(ctx, "mysql:8.4",
			mysql.WithDatabase("testdb"),
			mysql.WithUsername("user"),
			mysql.WithPassword("pass"),
		)
	}()
	if err != nil {
		t.Skipf("container start: %v", err)
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// create tables
	if _, err := db.ExecContext(ctx, `CREATE TABLE posts (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(32))`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE comments (id INT PRIMARY KEY AUTO_INCREMENT, message TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE custom_fields (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        table_name VARCHAR(255) NOT NULL,
        column_name VARCHAR(255) NOT NULL,
        data_type VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        UNIQUE KEY unique_table_column (table_name, column_name)
    )`); err != nil {
		t.Fatalf("create meta: %v", err)
	}

	sc := mysqlscanner.NewScanner(db)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "testdb"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	data, err := codec.EncodeYAML(metas)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// modify yaml: delete comments.message, update posts.name, add posts.age
	edited, err := codec.DecodeYAML(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var result []registry.FieldMeta
	for _, f := range edited {
		if f.TableName == "comments" && f.ColumnName == "message" {
			continue
		}
		if f.TableName == "posts" && f.ColumnName == "name" {
			f.DataType = "varchar(64)"
		}
		result = append(result, f)
	}
	result = append(result, registry.FieldMeta{TableName: "posts", ColumnName: "age", DataType: "int"})

	changes := registry.Diff(metas, result)
	var add, del, upd int
	var addUpd []registry.FieldMeta
	var dels []registry.FieldMeta
	for _, c := range changes {
		switch c.Type {
		case registry.ChangeAdded:
			add++
			addUpd = append(addUpd, *c.New)
		case registry.ChangeDeleted:
			del++
			dels = append(dels, *c.Old)
		case registry.ChangeUpdated:
			upd++
			addUpd = append(addUpd, *c.New)
		}
	}
	if add == 0 || del == 0 || upd == 0 {
		t.Fatalf("expected all change types")
	}
	if err := registry.DeleteSQL(ctx, db, "mysql", dels); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := registry.UpsertSQL(ctx, db, "mysql", addUpd); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	metas2, err := sc.Scan(ctx, registry.DBConfig{Schema: "testdb"})
	if err != nil {
		t.Fatalf("scan2: %v", err)
	}
	data2, err := codec.EncodeYAML(metas2)
	if err != nil {
		t.Fatalf("encode2: %v", err)
	}
	editedBytes, err := codec.EncodeYAML(result)
	if err != nil {
		t.Fatalf("encode edited: %v", err)
	}
	if string(data2) != string(editedBytes) {
		t.Fatalf("round trip mismatch\noriginal:\n%s\nexported:\n%s", string(editedBytes), string(data2))
	}
}
