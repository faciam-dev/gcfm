package registry

import "encoding/json"

// DisplayOption represents select options for display widgets.
type DisplayOption struct {
	Value    string `json:"value" yaml:"value"`
	LabelKey string `json:"labelKey" yaml:"labelKey"`
}

// DisplayMeta holds UI related metadata for a field.
type DisplayMeta struct {
	LabelKey       string          `json:"labelKey" yaml:"labelKey"`
	Widget         string          `json:"widget" yaml:"widget"`
	WidgetResolved string          `json:"widget_resolved,omitempty" yaml:"widget_resolved,omitempty"`
	PlaceholderKey string          `json:"placeholderKey" yaml:"placeholderKey"`
	Options        []DisplayOption `json:"options,omitempty" yaml:"options,omitempty"`
	WidgetConfig   json.RawMessage `json:"widget_config,omitempty" yaml:"widget_config,omitempty"`
}
