package mongo

import (
	"context"

	"github.com/faciam-dev/gcfm/internal/domain/capability"
)

// Adapter provides capability metadata for MongoDB-backed stores.
type Adapter struct{}

// New creates a MongoDB capability adapter.
func New() *Adapter {
	return &Adapter{}
}

// Capabilities returns the static capability definition for MongoDB drivers.
func (a *Adapter) Capabilities(_ context.Context, _ int64) (capability.Capabilities, error) {
	return capability.Capabilities{
		Driver: "mongodb",
		Types: []capability.Type{
			{Physical: "mongodb:string", Kind: "string"},
			{Physical: "mongodb:int", Kind: "integer"},
			{Physical: "mongodb:long", Kind: "integer"},
			{Physical: "mongodb:double", Kind: "number"},
			{Physical: "mongodb:decimal", Kind: "decimal"},
			{Physical: "mongodb:bool", Kind: "boolean"},
			{Physical: "mongodb:date", Kind: "datetime"},
			{Physical: "mongodb:timestamp", Kind: "datetime"},
			{Physical: "mongodb:object", Kind: "object"},
			{Physical: "mongodb:array", Kind: "array"},
			{Physical: "mongodb:objectId", Kind: "objectId"},
			{Physical: "mongodb:binary", Kind: "binary"},
			{Physical: "mongodb:uuid", Kind: "uuid"},
			{Physical: "mongodb:regex", Kind: "regex"},
		},
		Supports: capability.Supports{
			Default:      false,
			Required:     true,
			Unique:       true,
			TTL:          true,
			Geo:          true,
			PartialIndex: true,
		},
		Labels: capability.Labels{Table: "Collection", Column: "Field"},
	}, nil
}

func (a *Adapter) Scan(ctx context.Context, dbID int64) ([]capability.FieldSpec, error) {
	return nil, capability.ErrNotImplemented{Feature: "scan"}
}

func (a *Adapter) Plan(ctx context.Context, wanted []capability.FieldSpec) ([]capability.Op, error) {
	return nil, capability.ErrNotImplemented{Feature: "plan"}
}

func (a *Adapter) Apply(ctx context.Context, ops []capability.Op) error {
	return capability.ErrNotImplemented{Feature: "apply"}
}
