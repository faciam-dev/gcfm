package handlers

import (
	"net/http"
	"strconv"

	huma "github.com/danielgtaylor/huma/v2"

	"github.com/faciam-dev/gcfm/internal/service/plugins"
	"time"
)

// Authz defines capability checks used by handlers.
type Authz interface {
	HasCapability(ctx huma.Context, cap string) bool
}

// Config holds handler configuration.
type Config struct {
	PluginsMaxUploadMB int
}

// Handlers bundles dependencies for plugin upload endpoints.
type Handlers struct {
	Auth           Authz
	Cfg            Config
	PluginUploader *plugins.Uploader
}

// UploadPluginResponse represents response body.
type UploadPluginResponse struct {
	OK     bool      `json:"ok"`
	Widget WidgetDTO `json:"widget"`
}

// WidgetDTO mirrors the uploaded widget for API output.
type WidgetDTO struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Type         string         `json:"type"`
	Scopes       []string       `json:"scopes"`
	Enabled      bool           `json:"enabled"`
	Description  *string        `json:"description,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	Homepage     *string        `json:"homepage,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
	TenantScope  string         `json:"tenant_scope"`
	Tenants      []string       `json:"tenants"`
	UpdatedAt    string         `json:"updated_at"`
}

// RegisterPluginRoutes registers the upload endpoint.
func (h *Handlers) RegisterPluginRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "UploadPlugin",
		Method:      http.MethodPost,
		Path:        "/v1/plugins",
		Summary:     "Upload plugin",
		Tags:        []string{"Plugin"},
	}, func(ctx huma.Context) (*UploadPluginResponse, error) {
		if h.Auth != nil && !h.Auth.HasCapability(ctx, "plugins:write") {
			return nil, huma.NewError(huma.ErrForbidden, "missing capability plugins:write")
		}

		req := ctx.Request()
		maxMB := h.Cfg.PluginsMaxUploadMB
		if maxMB <= 0 {
			maxMB = 20
		}
		req.Body = http.MaxBytesReader(ctx.ResponseWriter(), req.Body, int64(maxMB)*1024*1024)
		if err := req.ParseMultipartForm(int64(maxMB) * 1024 * 1024); err != nil {
			return nil, huma.NewError(huma.ErrBadRequest, "invalid multipart form: "+err.Error())
		}

		file, fh, err := req.FormFile("file")
		if err != nil {
			return nil, huma.NewError(huma.ErrBadRequest, "file is required")
		}
		defer file.Close()

		tenantScope := req.FormValue("tenant_scope")
		tenants := req.MultipartForm.Value["tenants[]"]

		w, err := h.PluginUploader.HandleUpload(ctx, file, fh.Filename, plugins.UploadOptions{TenantScope: tenantScope, Tenants: tenants})
		if err != nil {
			code := http.StatusInternalServerError
			if plugins.IsClientErr(err) {
				code = http.StatusBadRequest
			}
			return nil, huma.NewError(&huma.ErrorDetail{Status: code, Title: http.StatusText(code), Detail: err.Error()})
		}

		dto := toWidgetDTO(w)
		ctx.Header().Set("Content-Type", "application/json")
		ctx.Header().Set("X-Uploaded-Size", strconv.FormatInt(w.PackageSize, 10))
		return &UploadPluginResponse{OK: true, Widget: dto}, nil
	})
}

func toWidgetDTO(w *plugins.UploadedWidget) WidgetDTO {
	return WidgetDTO{
		ID:           w.ID,
		Name:         w.Name,
		Version:      w.Version,
		Type:         w.Type,
		Scopes:       w.Scopes,
		Enabled:      w.Enabled,
		Description:  w.Description,
		Capabilities: w.Capabilities,
		Homepage:     w.Homepage,
		Meta:         w.Meta,
		TenantScope:  w.TenantScope,
		Tenants:      w.Tenants,
		UpdatedAt:    w.UpdatedAt.Format(time.RFC3339),
	}
}
