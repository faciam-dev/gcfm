package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/faciam-dev/gcfm/internal/customfield/widgetpolicy"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
)

type WidgetPolicyHandler struct {
	Store    *widgetpolicy.Store
	Registry widgetreg.Registry
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

func RegisterWidgetPolicy(api huma.API, h *WidgetPolicyHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "suggestWidgetPolicy",
		Method:      http.MethodGet,
		Path:        "/v1/widget-policies/suggest",
		Summary:     "Suggest widgets",
		Tags:        []string{"CustomField"},
	}, h.suggest)
}

func (h *WidgetPolicyHandler) suggest(ctx context.Context, in *suggestParams) (*suggestOutput, error) {
	base, length, enums := widgetpolicy.ParseTypeInfo(in.Type)
	var lptr *int
	if in.Length > 0 {
		lptr = &in.Length
	} else {
		lptr = length
	}
	pctx := widgetpolicy.AutoResolveCtx{Driver: in.Driver, Type: base, Validator: in.Validator, Length: lptr, ColumnName: in.Name, EnumValues: enums}
	hasPlugin := func(id string) bool {
		if strings.HasPrefix(id, "plugin://") {
			pid := strings.TrimPrefix(id, "plugin://")
			return h.Registry == nil || h.Registry.Has(pid)
		}
		return true
	}
	pol := h.Store.Policy()
	id, cfg := pol.Resolve(pctx, hasPlugin)
	suggIDs := pol.Suggest(pctx, hasPlugin)
	out := &suggestOutput{}
	out.Body.Resolved.ID = id
	if len(cfg) > 0 {
		out.Body.Resolved.Config = cfg
	}
	for _, sid := range suggIDs {
		label := sid
		if sid == "core://auto" {
			label = "System default"
		} else if strings.HasPrefix(sid, "plugin://") {
			label = strings.TrimPrefix(sid, "plugin://")
		}
		out.Body.Suggested = append(out.Body.Suggested, suggestion{ID: sid, Label: label})
	}
	return out, nil
}
