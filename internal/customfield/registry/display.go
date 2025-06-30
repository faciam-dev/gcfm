package registry

// DisplayOption represents select options for display widgets.
type DisplayOption struct {
	Value    string `json:"value" yaml:"value"`
	LabelKey string `json:"labelKey" yaml:"labelKey"`
}

// DisplayMeta holds UI related metadata for a field.
type DisplayMeta struct {
	LabelKey       string          `json:"labelKey" yaml:"labelKey"`
	Widget         string          `json:"widget" yaml:"widget"`
	PlaceholderKey string          `json:"placeholderKey" yaml:"placeholderKey"`
	Options        []DisplayOption `json:"options,omitempty" yaml:"options,omitempty"`
}
