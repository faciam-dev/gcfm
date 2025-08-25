package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

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
		var spec bson.M
		if err := cur.Decode(&spec); err != nil {
			return nil, err
		}
		name, _ := spec["name"].(string)
		coll := db.Collection(name)

		var (
			colMetas []registry.FieldMeta
			metaMap  = make(map[string]*registry.FieldMeta)
		)
		if opts, ok := spec["options"].(bson.M); ok {
			if validator, ok := opts["validator"].(bson.M); ok {
				if schema, ok := validator["$jsonSchema"].(bson.M); ok {
					required := make(map[string]struct{})
					if req, ok := schema["required"].(bson.A); ok {
						for _, v := range req {
							if s, ok := v.(string); ok {
								required[s] = struct{}{}
							}
						}
					}
					if props, ok := schema["properties"].(bson.M); ok {
						for field, v := range props {
							if m, ok := v.(bson.M); ok {
								fm := registry.FieldMeta{
									TableName:  name,
									ColumnName: field,
									DataType:   mapBSONType(m["bsonType"]),
									Nullable:   true,
								}
								if _, ok := required[field]; ok {
									fm.Nullable = false
								}
								if def, ok := m["default"]; ok {
									fm.HasDefault = true
									v := fmt.Sprint(def)
									fm.Default = &v
								}
								colMetas = append(colMetas, fm)
								metaMap[field] = &colMetas[len(colMetas)-1]
							}
						}
					}
				}
			}
		}
		if len(colMetas) == 0 {
			samples, err := sampleCollection(ctx, name, coll)
			if err != nil {
				return nil, err
			}
			colMetas = samples
		}
		// unique indexes
		idxCur, err := coll.Indexes().List(ctx)
		if err == nil {
			for idxCur.Next(ctx) {
				var idx bson.M
				if err := idxCur.Decode(&idx); err == nil {
					if u, ok := idx["unique"].(bool); ok && u {
						if key, ok := idx["key"].(bson.M); ok && len(key) == 1 {
							for field := range key {
								if m, ok := metaMap[field]; ok {
									m.Unique = true
								} else {
									fm := registry.FieldMeta{TableName: name, ColumnName: field, DataType: "string", Unique: true, Nullable: true}
									colMetas = append(colMetas, fm)
								}
							}
						}
					}
				}
			}
			idxCur.Close(ctx)
		}

		metas = append(metas, colMetas...)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return metas, nil
}

func sampleCollection(ctx context.Context, name string, coll *mongo.Collection) ([]registry.FieldMeta, error) {
	cur, err := coll.Find(ctx, bson.D{}, options.Find().SetLimit(10))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	typeMap := make(map[string]map[string]struct{})
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		for k, v := range doc {
			t := inferType(v)
			if _, ok := typeMap[k]; !ok {
				typeMap[k] = make(map[string]struct{})
			}
			typeMap[k][t] = struct{}{}
		}
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	metas := make([]registry.FieldMeta, 0, len(typeMap))
	for k, types := range typeMap {
		if len(types) == 1 {
			for t := range types {
				metas = append(metas, registry.FieldMeta{TableName: name, ColumnName: k, DataType: t})
			}
		} else {
			metas = append(metas, registry.FieldMeta{TableName: name, ColumnName: k, DataType: "string"})
		}
	}
	return metas, nil
}

func inferType(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case int32, int16, int8, int:
		return "int"
	case int64:
		return "long"
	case float64, float32:
		return "double"
	case primitive.Decimal128:
		return "decimal"
	case bool:
		return "bool"
	case primitive.DateTime, time.Time:
		return "date"
	default:
		return "string"
	}
}

func mapBSONType(v interface{}) string {
	switch t := v.(type) {
	case string:
		return translateBSONType(t)
	case []interface{}:
		if len(t) == 1 {
			if s, ok := t[0].(string); ok {
				return translateBSONType(s)
			}
		}
	}
	return "string"
}

func translateBSONType(t string) string {
	switch t {
	case "double":
		return "double"
	case "string":
		return "string"
	case "bool", "boolean":
		return "bool"
	case "int", "int32":
		return "int"
	case "long", "int64":
		return "long"
	case "decimal", "decimal128":
		return "decimal"
	case "date":
		return "date"
	default:
		return "string"
	}
}
