package schema

import "encoding/json"

type DisplaySettings struct {
	LabelKey       *string         `json:"labelKey,omitempty"`
	Widget         string          `json:"widget"`
	PlaceholderKey *string         `json:"placeholderKey,omitempty"`
	WidgetConfig   json.RawMessage `json:"widgetConfig,omitempty"`
}

type CustomField struct {
	DBID            *int64           `json:"db_id,omitempty" validate:"omitempty,min=1"`
	Table           string           `json:"table"`
	Column          string           `json:"column"`
	Type            string           `json:"type"`
	Display         *DisplaySettings `json:"display,omitempty" validate:"omitempty"`
	Nullable        *bool            `json:"nullable,omitempty" validate:"omitempty"`
	Unique          *bool            `json:"unique,omitempty" validate:"omitempty"`
	HasDefault      bool             `json:"hasDefault,omitempty"`
	DefaultValue    *string          `json:"defaultValue,omitempty" validate:"omitempty"`
	Validator       string           `json:"validator,omitempty" validate:"omitempty"`
	ValidatorParams map[string]any   `json:"validator_params,omitempty" validate:"omitempty"`
}
