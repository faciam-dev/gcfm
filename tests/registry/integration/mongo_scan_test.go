package integration_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	mscanner "github.com/faciam-dev/gcfm/internal/driver/mongo"
)

func TestMongoScan(t *testing.T) {
	ctx := context.Background()
	t.Skip("integration test requires Docker")
	container, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Skipf("container: %v", err)
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("uri: %v", err)
	}
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cli.Disconnect(ctx)

	coll := cli.Database("appdb").Collection("users")
	if _, err := coll.InsertOne(ctx, bson.M{"name": "a"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	sc := mscanner.NewScanner(cli)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "appdb"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(metas) == 0 {
		t.Fatalf("no metas")
	}
}
