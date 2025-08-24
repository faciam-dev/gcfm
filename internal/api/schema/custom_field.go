package schema

import "github.com/faciam-dev/gcfm/internal/customfield/registry"

type CustomField struct {
	DBID            *int64                `json:"db_id,omitempty" validate:"omitempty,min=1"`
	Table           string                `json:"table"`
	Column          string                `json:"column"`
	Type            string                `json:"type"`
	Display         *registry.DisplayMeta `json:"display,omitempty" validate:"omitempty"`
	Nullable        *bool                 `json:"nullable,omitempty" validate:"omitempty"`
	Unique          *bool                 `json:"unique,omitempty" validate:"omitempty"`
	HasDefault      bool                  `json:"hasDefault,omitempty"`
	DefaultValue    *string               `json:"defaultValue,omitempty" validate:"omitempty"`
	Validator       string                `json:"validator,omitempty" validate:"omitempty"`
	ValidatorParams map[string]any        `json:"validator_params,omitempty" validate:"omitempty"`
}
