package widgetpolicy

import "strings"

type Ctx struct {
	Driver     string
	Type       string
	Validator  string
	Length     *int
	Name       string
	EnumValues []string
}

func (p *WidgetPolicy) Resolve(ctx Ctx, hasPlugin func(string) bool) (id string, cfg map[string]any) {
	for _, r := range p.Rules {
		if match(r.When, ctx) {
			id, cfg = r.Widget, renderConfig(r.Config, ctx)
			if !hasPlugin(id) && !strings.HasPrefix(id, "core://") {
				id, cfg = "plugin://text-input", map[string]any{}
			}
			return
		}
	}
	return "plugin://text-input", map[string]any{}
}

func (p *WidgetPolicy) Suggest(ctx Ctx) []string {
	seen := map[string]bool{}
	out := []string{"core://auto"}
	for _, r := range p.Rules {
		if match(r.When, ctx) {
			if !seen[r.Widget] {
				out = append(out, r.Widget)
				seen[r.Widget] = true
			}
			if r.Stop && len(out)-1 >= p.SuggestTop {
				break
			}
		}
	}
	if len(out) == 1 {
		out = append(out, "plugin://text-input")
	}
	return out
}

func match(w RuleWhen, c Ctx) bool {
	in := func(list []string, v string) bool {
		if len(list) == 0 {
			return true
		}
		v = strings.ToLower(v)
		for _, x := range list {
			if strings.ToLower(x) == v {
				return true
			}
		}
		return false
	}
	if !in(w.Driver, c.Driver) {
		return false
	}
	if !in(w.Validator, c.Validator) {
		return false
	}
	if !in(w.Types, c.Type) {
		return false
	}
	if w.LengthMin != nil && (c.Length == nil || *c.Length < *w.LengthMin) {
		return false
	}
	if w.LengthMax != nil && (c.Length != nil && *c.Length > *w.LengthMax) {
		return false
	}
	if w.rx != nil && !w.rx.MatchString(c.Name) {
		return false
	}
	return true
}
