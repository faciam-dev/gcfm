package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	huma "github.com/faciam-dev/gcfm/internal/huma"
	"github.com/faciam-dev/gcfm/internal/service/plugins"
)

// Authz defines capability checks used by handlers.
type Authz interface {
	HasCapability(ctx context.Context, cap string) bool
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

// PluginUploadForm describes the expected multipart form fields.
type PluginUploadForm struct {
	File        huma.FormFile `form:"file" required:"true" doc:"ZIP/TGZ package"`
	TenantScope string        `form:"tenant_scope,omitempty" doc:"system|tenant"`
	Tenants     []string      `form:"tenants" doc:"限定テナント(繰り返し可)"`
}

type uploadPluginOutput struct {
	Body    UploadPluginResponse
	Headers struct {
		XUploadedSize string `header:"X-Uploaded-Size"`
	}
}

// RegisterPluginRoutes registers the upload endpoint.
func (h *Handlers) RegisterPluginRoutes(api huma.API) {
	op := huma.Operation{
		OperationID: "UploadPlugin",
		Method:      http.MethodPost,
		Path:        "/v1/plugins",
		Summary:     "Upload a plugin package (ZIP/TGZ)",
		Description: "Accepts multipart/form-data with a file field named 'file'.",
		Tags:        []string{"Plugin"},
	}
	huma.RegisterConsumes(api, op, []string{huma.ContentTypeMultipartForm}, h.uploadPlugin)
}

func (h *Handlers) uploadPlugin(ctx context.Context, form *PluginUploadForm) (*uploadPluginOutput, error) {
	if h.Auth != nil && !h.Auth.HasCapability(ctx, "plugins:write") {
		return nil, huma.NewError(http.StatusForbidden, "missing capability plugins:write", nil)
	}

	maxMB := h.Cfg.PluginsMaxUploadMB
	if maxMB <= 0 {
		maxMB = 20
	}

	if form.File.File == nil || form.File.Size == 0 {
		return nil, huma.NewError(http.StatusBadRequest, "file is required", nil)
	}
	if form.File.Size > int64(maxMB)*1024*1024 {
		return nil, huma.NewError(http.StatusBadRequest, "file too large", nil)
	}

	f, err := form.File.Open()
	if err != nil {
		return nil, huma.NewError(http.StatusBadRequest, "cannot open file: "+err.Error(), nil)
	}
	defer f.Close()

	w, err := h.PluginUploader.HandleUpload(ctx, f, form.File.Filename, plugins.UploadOptions{
		TenantScope: form.TenantScope,
		Tenants:     form.Tenants,
	})
	if err != nil {
		if plugins.IsClientErr(err) {
			return nil, huma.NewError(http.StatusBadRequest, err.Error(), nil)
		}
		return nil, huma.NewError(http.StatusInternalServerError, err.Error(), nil)
	}

	dto := toWidgetDTO(w)
	out := &uploadPluginOutput{Body: UploadPluginResponse{OK: true, Widget: dto}}
	out.Headers.XUploadedSize = strconv.FormatInt(w.PackageSize, 10)
	return out, nil
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
