package handlers

import (
	"context"
	"mime/multipart"
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

// uploadPluginInput defines expected multipart form fields.
type uploadPluginInput struct {
	File        *multipart.FileHeader `form:"file" json:"file" required:"true"`
	TenantScope string                `form:"tenant_scope" json:"tenant_scope"`
	Tenants     []string              `form:"tenants" json:"tenants"`
}

type uploadPluginOutput struct {
	Body    UploadPluginResponse
	Headers struct {
		XUploadedSize string `header:"X-Uploaded-Size"`
	}
}

// RegisterPluginRoutes registers the upload endpoint.
func (h *Handlers) RegisterPluginRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "UploadPlugin",
		Method:      http.MethodPost,
		Path:        "/v1/plugins",
		Summary:     "Upload plugin",
		Tags:        []string{"Plugin"},
	}, h.uploadPlugin)
}

func (h *Handlers) uploadPlugin(ctx context.Context, in *uploadPluginInput) (*uploadPluginOutput, error) {
	if h.Auth != nil && !h.Auth.HasCapability(ctx, "plugins:write") {
		return nil, huma.NewError(http.StatusForbidden, "missing capability plugins:write", nil)
	}

	maxMB := h.Cfg.PluginsMaxUploadMB
	if maxMB <= 0 {
		maxMB = 20
	}

	if in.File == nil {
		return nil, huma.NewError(http.StatusBadRequest, "file is required", nil)
	}
	if in.File.Size > int64(maxMB)*1024*1024 {
		return nil, huma.NewError(http.StatusBadRequest, "file too large", nil)
	}

	f, err := in.File.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	w, err := h.PluginUploader.HandleUpload(ctx, f, in.File.Filename, plugins.UploadOptions{
		TenantScope: in.TenantScope,
		Tenants:     in.Tenants,
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
