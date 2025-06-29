package registry

// DisplayOption represents select options for display widgets.
type DisplayOption struct {
	Value    string `yaml:"value"`
	LabelKey string `yaml:"labelKey"`
}

// DisplayMeta holds UI related metadata for a field.
type DisplayMeta struct {
	LabelKey       string          `yaml:"labelKey"`
	Widget         string          `yaml:"widget"`
	PlaceholderKey string          `yaml:"placeholderKey"`
	Options        []DisplayOption `yaml:"options,omitempty"`
}
