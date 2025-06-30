package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/faciam-dev/gcfm/internal/metadata"
)

// TableLister lists MongoDB collections.
type TableLister struct{ client *mongo.Client }

// NewTableLister returns a new TableLister.
func NewTableLister(cli *mongo.Client) *TableLister { return &TableLister{client: cli} }

// ListTables returns collections for the given schema.
func (l *TableLister) ListTables(ctx context.Context, schema string) ([]metadata.Table, error) {
	cur, err := l.client.Database(schema).ListCollections(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var tables []metadata.Table
	for cur.Next(ctx) {
		var info struct {
			Name    string `bson:"name"`
			Options struct {
				Comment string `bson:"comment,omitempty"`
			} `bson:"options,omitempty"`
		}
		if err := cur.Decode(&info); err != nil {
			return nil, err
		}
		tables = append(tables, metadata.Table{Name: info.Name, Comment: info.Options.Comment})
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}
