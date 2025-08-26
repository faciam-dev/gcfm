package handler

import (
	"context"
	"net/http"
	"strings"

	huma "github.com/faciam-dev/gcfm/internal/huma"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
	"github.com/faciam-dev/gcfm/pkg/widgetpolicy"
)

type WidgetPolicyHandler struct {
	Store      *widgetpolicy.Store
	Registry   widgetreg.Registry
	PolicyPath string
}

type suggestParams struct {
	Driver    string `query:"driver"`
	Type      string `query:"type"`
	Validator string `query:"validator"`
	Length    int    `query:"length"`
	Name      string `query:"name"`
}

type suggestion struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type suggestOutput struct {
	Body struct {
		Resolved struct {
			ID     string         `json:"id"`
			Config map[string]any `json:"config,omitempty"`
		} `json:"resolved"`
		Suggested []suggestion `json:"suggested"`
	}
}

type statusOutput struct {
	Body struct {
		Path       string                    `json:"path"`
		Rules      int                       `json:"rules"`
		SuggestTop int                       `json:"suggest_top"`
		Sample     []widgetpolicy.PolicyRule `json:"sample"`
	}
}

func RegisterWidgetPolicy(api huma.API, h *WidgetPolicyHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "suggestWidgetPolicy",
		Method:      http.MethodGet,
		Path:        "/v1/widget-policies/suggest",
		Summary:     "Suggest widgets",
		Tags:        []string{"CustomField"},
	}, h.suggest)
	huma.Register(api, huma.Operation{
		OperationID: "widgetPolicyStatus",
		Method:      http.MethodGet,
		Path:        "/v1/widget-policies/_status",
		Summary:     "Widget policy status",
		Tags:        []string{"CustomField"},
	}, h.status)
}

func (h *WidgetPolicyHandler) suggest(ctx context.Context, in *suggestParams) (*suggestOutput, error) {
	base, lengthParsed, enums := widgetpolicy.ParseTypeInfo(in.Type)
	var length *int
	if in.Length > 0 {
		l := int(in.Length)
		length = &l
	} else {
		length = lengthParsed
	}
	typ, _ := widgetpolicy.NormalizeType(strings.ToLower(in.Driver), base, length)
	val := widgetpolicy.NormalizeValidator(in.Validator)
	pctx := widgetpolicy.Ctx{Driver: strings.ToLower(in.Driver), Type: typ, Validator: val, Length: length, Name: in.Name, EnumValues: enums}
	hasPlugin := func(id string) bool {
		if strings.HasPrefix(id, "plugin://") {
			pid := strings.TrimPrefix(id, "plugin://")
			return h.Registry == nil || h.Registry.Has(pid)
		}
		return true
	}
	pol := h.Store.Get()
	id, cfg := pol.Resolve(pctx, hasPlugin)
	suggIDs := pol.Suggest(pctx)
	out := &suggestOutput{}
	out.Body.Resolved.ID = id
	if len(cfg) > 0 {
		out.Body.Resolved.Config = cfg
	}
	for _, sid := range suggIDs {
		out.Body.Suggested = append(out.Body.Suggested, suggestion{ID: sid, Label: labelFromID(sid)})
	}
	return out, nil
}

func (h *WidgetPolicyHandler) status(ctx context.Context, _ *struct{}) (*statusOutput, error) {
	p := h.Store.Get()
	sample := p.Rules
	if len(sample) > 3 {
		sample = sample[:3]
	}
	out := &statusOutput{}
	out.Body.Path = h.PolicyPath
	out.Body.Rules = len(p.Rules)
	out.Body.SuggestTop = p.SuggestTop
	out.Body.Sample = sample
	return out, nil
}

func labelFromID(id string) string {
	switch id {
	case "core://auto":
		return "System default"
	case "plugin://text-input":
		return "text-input"
	case "plugin://email-input":
		return "Email Input"
	case "plugin://url-input":
		return "URL Input"
	case "plugin://uuid-input":
		return "UUID Input"
	case "plugin://password-input":
		return "Password"
	case "plugin://number-input":
		return "Number"
	case "plugin://date-input":
		return "Date"
	case "plugin://time-input":
		return "Time"
	case "plugin://datetime-input":
		return "Datetime"
	case "plugin://json-editor":
		return "JSON Editor"
	case "plugin://select":
		return "Select"
	case "plugin://multiselect":
		return "Multi Select"
	default:
		return strings.TrimPrefix(id, "plugin://")
	}
}
