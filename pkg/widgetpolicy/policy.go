package widgetpolicy

import (
	"regexp"
	"strings"
)

type WidgetPolicy struct {
	Version    int          `yaml:"version" json:"version"`
	SuggestTop int          `yaml:"suggest_top" json:"suggest_top"`
	Rules      []PolicyRule `yaml:"rules" json:"rules"`
}

type PolicyRule struct {
	ID     string         `yaml:"id" json:"id"`
	When   RuleWhen       `yaml:"when" json:"when"`
	Widget string         `yaml:"widget" json:"widget"`
	Config map[string]any `yaml:"config" json:"config"`
	Stop   bool           `yaml:"stop" json:"stop"`
}

type RuleWhen struct {
	Types     []string `yaml:"types" json:"types"`
	Validator []string `yaml:"validator" json:"validator"`
	Driver    []string `yaml:"driver" json:"driver"`
	LengthMin *int     `yaml:"length_min" json:"length_min"`
	LengthMax *int     `yaml:"length_max" json:"length_max"`
	NameRegex string   `yaml:"name_regex" json:"name_regex"`

	rx *regexp.Regexp
}

func (p *WidgetPolicy) Normalize() {
	low := func(ss []string) []string {
		r := make([]string, len(ss))
		for i, s := range ss {
			r[i] = strings.ToLower(strings.TrimSpace(s))
		}
		return r
	}
	for i := range p.Rules {
		r := &p.Rules[i]
		r.Widget = strings.TrimSpace(r.Widget)
		r.When.Types = low(r.When.Types)
		r.When.Validator = low(r.When.Validator)
		r.When.Driver = low(r.When.Driver)
		if r.When.NameRegex != "" {
			r.When.rx = regexp.MustCompile(r.When.NameRegex)
		}
	}
}
