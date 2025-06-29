package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

type Scanner struct {
	client *mongo.Client
}

func NewScanner(client *mongo.Client) *Scanner {
	return &Scanner{client: client}
}

func (s *Scanner) Scan(ctx context.Context, conf registry.DBConfig) ([]registry.FieldMeta, error) {
	db := s.client.Database(conf.Schema)
	cur, err := db.ListCollections(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var metas []registry.FieldMeta
	for cur.Next(ctx) {
		var col struct {
			Name string `bson:"name"`
		}
		if err := cur.Decode(&col); err != nil {
			return nil, err
		}
		coll := db.Collection(col.Name)
		var doc bson.M
		err := coll.FindOne(ctx, bson.D{}).Decode(&doc)
		if err == mongo.ErrNoDocuments {
			continue
		}
		if err != nil {
			return nil, err
		}
		for k := range doc {
			// All MongoDB fields are currently assumed to be of type "string".
			// TODO: implement dynamic type inference via $jsonSchema.
			metas = append(metas, registry.FieldMeta{TableName: col.Name, ColumnName: k, DataType: "string"})
		}
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return metas, nil
}
