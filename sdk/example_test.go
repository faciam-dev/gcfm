package sdk_test

import (
	"context"
	"log"

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
