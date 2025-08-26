//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	mscanner "github.com/faciam-dev/gcfm/pkg/driver/mongo"
	"github.com/faciam-dev/gcfm/pkg/registry"
)

func TestMongoScanMetadata(t *testing.T) {
	ctx := context.Background()
	container, err := func() (c *mongodb.MongoDBContainer, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		return mongodb.Run(ctx, "mongo:7")
	}()
	if err != nil {
		t.Skipf("container: %v", err)
	}
	if container == nil {
		t.Fatalf("container is nil")
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

	db := cli.Database("appdb")
	cmd := bson.D{{"create", "users"}, {"validator", bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": bson.A{"email"},
		"properties": bson.M{
			"email":    bson.M{"bsonType": "string"},
			"age":      bson.M{"bsonType": "int"},
			"nickname": bson.M{"bsonType": "string", "default": "guest"},
		},
	}}}}
	if err := db.RunCommand(ctx, cmd).Err(); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	coll := db.Collection("users")
	if _, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{"email", 1}}, Options: options.Index().SetUnique(true)}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	sc := mscanner.NewScanner(cli)
	metas, err := sc.Scan(ctx, registry.DBConfig{Schema: "appdb"})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	find := func(col string) registry.FieldMeta {
		for _, m := range metas {
			if m.TableName == "users" && m.ColumnName == col {
				return m
			}
		}
		t.Fatalf("column %s not found", col)
		return registry.FieldMeta{}
	}
	email := find("email")
	if email.Nullable || !email.Unique || email.HasDefault {
		t.Fatalf("email meta incorrect: %+v", email)
	}
	age := find("age")
	if !age.Nullable || age.Unique || age.HasDefault {
		t.Fatalf("age meta incorrect: %+v", age)
	}
	nick := find("nickname")
	if !nick.Nullable || nick.Unique || !nick.HasDefault || nick.Default == nil || *nick.Default != "guest" {
		t.Fatalf("nickname meta incorrect: %+v", nick)
	}
}
