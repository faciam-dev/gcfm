package widgetpolicy

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

func renderConfig(in map[string]any, ctx Ctx) map[string]any {
	if in == nil {
		return nil
	}
	out := map[string]any{}
	fm := template.FuncMap{
		"join": strings.Join,
		"or": func(a, b any) any {
			if s, ok := a.(string); ok && s == "" {
				return b
			}
			if a == nil {
				return b
			}
			return a
		},
		"eq": func(a, b any) bool { return fmt.Sprint(a) == fmt.Sprint(b) },
	}
	for k, v := range in {
		s, ok := v.(string)
		if !ok {
			out[k] = v
			continue
		}
		tpl, err := template.New("cfg").Funcs(fm).Parse(s)
		if err != nil {
			out[k] = v
			continue
		}
		var buf bytes.Buffer
		_ = tpl.Execute(&buf, map[string]any{
			"Driver":     ctx.Driver,
			"Type":       ctx.Type,
			"Validator":  ctx.Validator,
			"Length":     ctx.Length,
			"Name":       ctx.Name,
			"EnumValues": ctx.EnumValues,
		})
		out[k] = buf.String()
	}
	return out
}
