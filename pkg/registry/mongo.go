package registry

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func LoadMongo(ctx context.Context, cli *mongo.Client, conf DBConfig) ([]FieldMeta, error) {
	coll := cli.Database(conf.Schema).Collection("custom_fields")
	cur, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var metas []FieldMeta
	for cur.Next(ctx) {
		var m FieldMeta
		if err := cur.Decode(&m); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return metas, nil
}

func UpsertMongo(ctx context.Context, cli *mongo.Client, conf DBConfig, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	coll := cli.Database(conf.Schema).Collection("custom_fields")
	for _, m := range metas {
		filter := bson.M{"table_name": m.TableName, "column_name": m.ColumnName}
		update := bson.M{"$set": bson.M{"data_type": m.DataType, "table_name": m.TableName, "column_name": m.ColumnName}}
		if _, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}

func DeleteMongo(ctx context.Context, cli *mongo.Client, conf DBConfig, metas []FieldMeta) error {
	if len(metas) == 0 {
		return nil
	}
	coll := cli.Database(conf.Schema).Collection("custom_fields")
	for _, m := range metas {
		if _, err := coll.DeleteOne(ctx, bson.M{"table_name": m.TableName, "column_name": m.ColumnName}); err != nil {
			return err
		}
	}
	return nil
}
