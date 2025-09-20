package schema

import "encoding/json"

type DisplaySettings struct {
	LabelKey       *string         `json:"labelKey,omitempty"`
	Widget         string          `json:"widget"`
	WidgetResolved string          `json:"widget_resolved,omitempty"`
	PlaceholderKey *string         `json:"placeholderKey,omitempty"`
	WidgetConfig   json.RawMessage `json:"widget_config,omitempty"`
}

type DefaultSpec struct {
	Mode                     string `json:"mode" enum:"none,literal,expression" doc:"Default value mode"`
	Raw                      string `json:"raw,omitempty" doc:"Literal or expression text, unquoted"`
	OnUpdateCurrentTimestamp *bool  `json:"on_update_current_timestamp,omitempty"`
}

type CustomField struct {
	DBID         *int64          `json:"db_id,omitempty" validate:"omitempty,min=1"`
	Table        string          `json:"table"`
	Column       string          `json:"column"`
	Type         string          `json:"type"`
	StoreKind    *string         `json:"store_kind,omitempty"`
	Kind         *string         `json:"kind,omitempty"`
	PhysicalType *string         `json:"physical_type,omitempty"`
	DriverExtras map[string]any  `json:"driver_extras,omitempty"`
	Display      DisplaySettings `json:"display"`
	Nullable     *bool           `json:"nullable,omitempty" validate:"omitempty"`
	Unique       *bool           `json:"unique,omitempty" validate:"omitempty"`
	HasDefault   bool            `json:"hasDefault,omitempty"`                        // DEPRECATED: use default.mode
	DefaultValue *string         `json:"defaultValue,omitempty" validate:"omitempty"` // DEPRECATED: use default.raw
	Default      *DefaultSpec    `json:"default,omitempty"`

	// ignored compatibility fields for FE
	DbDriver                     *string        `json:"db_driver,omitempty" doc:"Ignored. Server detects driver."`
	DefaultMode                  *string        `json:"default_mode,omitempty" doc:"Ignored. Use default.mode"`
	DefaultRaw                   *string        `json:"default_raw,omitempty" doc:"Ignored. Use default.raw"`
	OnUpdateCurrentTimestampFlat *bool          `json:"on_update_current_timestamp,omitempty" doc:"Ignored. Use default.on_update_current_timestamp"`
	Validator                    string         `json:"validator,omitempty" validate:"omitempty"`
	ValidatorParams              map[string]any `json:"validator_params,omitempty" validate:"omitempty"`
}
