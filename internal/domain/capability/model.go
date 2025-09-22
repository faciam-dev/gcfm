package capability

import "context"

// Type describes a driver-specific physical type and its logical counterpart.
type Type struct {
	Physical string `json:"physical"`
	Kind     string `json:"kind"`
}

// Supports enumerates opt-in features supported by a driver.
type Supports struct {
	Default      bool `json:"default"`
	Required     bool `json:"required"`
	Unique       bool `json:"unique"`
	TTL          bool `json:"ttl"`
	Geo          bool `json:"geo"`
	PartialIndex bool `json:"partialIndex"`
}

// Labels customizes terminology for UI display.
type Labels struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// Capabilities aggregates the type matrix and feature support for a driver.
type Capabilities struct {
	Driver   string   `json:"driver"`
	Types    []Type   `json:"types"`
	Supports Supports `json:"supports"`
	Labels   Labels   `json:"labels"`
}

// FieldSpec represents a desired field definition for planning operations.
type FieldSpec struct {
	DBID       int64          `json:"dbId"`
	Collection string         `json:"collection"`
	Field      string         `json:"field"`
	StoreKind  string         `json:"storeKind"`
	Kind       string         `json:"kind"`
	Physical   string         `json:"physical"`
	Extras     map[string]any `json:"extras,omitempty"`
}

// Op describes a low-level operation required to align the target store.
type Op struct {
	Op         string         `json:"op"`
	Collection string         `json:"collection"`
	Payload    map[string]any `json:"payload"`
}

// Adapter defines driver-specific behaviour for capabilities and schema ops.
type Adapter interface {
	Capabilities(ctx context.Context, dbID int64) (Capabilities, error)
	Scan(ctx context.Context, dbID int64) ([]FieldSpec, error)
	Plan(ctx context.Context, wanted []FieldSpec) ([]Op, error)
	Apply(ctx context.Context, ops []Op) error
}

// ErrNotImplemented is returned by adapters for unsupported operations.
type ErrNotImplemented struct{ Feature string }

func (e ErrNotImplemented) Error() string {
	if e.Feature == "" {
		return "feature not implemented"
	}
	return e.Feature + " not implemented"
}
