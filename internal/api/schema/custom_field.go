package schema

import "github.com/faciam-dev/gcfm/internal/customfield/registry"

type CustomField struct {
	Table     string                `json:"table"`
	Column    string                `json:"column"`
	Type      string                `json:"type"`
	Display   *registry.DisplayMeta `json:"display,omitempty" validate:"omitempty"`
	Nullable  *bool                 `json:"nullable,omitempty" validate:"omitempty"`
	Unique    *bool                 `json:"unique,omitempty" validate:"omitempty"`
	Default   *string               `json:"default,omitempty" validate:"omitempty"`
	Validator string                `json:"validator,omitempty" validate:"omitempty"`
}
