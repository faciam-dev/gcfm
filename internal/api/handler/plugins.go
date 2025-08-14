package handler

import (
	"context"
	"net/http"

	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/plugin"
)

// PluginHandler handles plugin APIs.
type PluginHandler struct {
	UC plugin.Usecase
}

type pluginListOutput struct {
	Body []plugin.Plugin `json:"plugins"`
}

// RegisterPlugins registers the /v1/plugins endpoint.
func RegisterPlugins(api huma.API, h *PluginHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listPlugins",
		Method:      http.MethodGet,
		Path:        "/v1/plugins",
		Summary:     "List plugins",
		Tags:        []string{"Plugin"},
	}, h.list)
}

func (h *PluginHandler) list(ctx context.Context, _ *struct{}) (*pluginListOutput, error) {
	ps, err := h.UC.List(ctx)
	if err != nil {
		return nil, err
	}
	return &pluginListOutput{Body: ps}, nil
}
