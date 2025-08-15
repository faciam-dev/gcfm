package sdk_test

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/faciam-dev/gcfm/sdk"
)

func ExampleService_quickstart() {
	ctx := context.Background()
	svc := sdk.New(sdk.ServiceConfig{})
	cfg := sdk.DBConfig{
		DSN:    "mysql://user:pass@tcp(localhost:3306)/app",
		Schema: "app",
	}

	yaml, err := svc.Export(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := svc.Apply(ctx, cfg, yaml, sdk.ApplyOptions{DryRun: true}); err != nil {
		log.Fatal(err)
	}

	if err := svc.MigrateRegistry(ctx, cfg, 0); err != nil {
		log.Fatal(err)
	}
}

func ExampleService_separateMetaDB() {
	meta, _ := sql.Open("postgres", os.Getenv("META_DSN"))
	target, _ := sql.Open("mysql", os.Getenv("TARGET_DSN"))
	_ = sdk.New(sdk.ServiceConfig{
		DB:         target,
		Driver:     "mysql",
		MetaDB:     meta,
		MetaDriver: "postgres",
		MetaSchema: "gcfm_meta",
	})
}

func ExampleService_multiTarget() {
	meta, _ := sql.Open("postgres", os.Getenv("META_DSN"))
	dbA, _ := sql.Open("mysql", os.Getenv("TENANT_A_DSN"))
	dbB, _ := sql.Open("mysql", os.Getenv("TENANT_B_DSN"))

	svc := sdk.New(sdk.ServiceConfig{
		DB:         dbA,
		Driver:     "mysql",
		MetaDB:     meta,
		MetaDriver: "postgres",
		MetaSchema: "gcfm_meta",
		Targets: []sdk.TargetConfig{
			{Key: "tenant:A", DB: dbA, Driver: "mysql", Schema: ""},
			{Key: "tenant:B", DB: dbB, Driver: "mysql", Schema: ""},
		},
		TargetResolver: sdk.TenantResolverFromPrefix("tenant:"),
	})

	ctx := sdk.WithTenantID(context.Background(), "A")
	_, _ = svc.ListCustomFields(ctx, 1, "posts")
}
