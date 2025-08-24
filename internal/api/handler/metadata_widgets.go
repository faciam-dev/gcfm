package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	humago "github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/gcfm/internal/registry/widgets"
	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"github.com/faciam-dev/gcfm/internal/util"
)

type WidgetNotifier interface {
	NotifyWidgetChanged(ctx context.Context, id string) error
	NotifyWidgetRemoved(ctx context.Context, id string) error
}

type Authz interface {
	HasCapability(ctx context.Context, cap string) bool
}

type WidgetHandler struct {
	Reg      widgets.Registry
	Repo     widgetsrepo.Repo
	Notifier WidgetNotifier
	Auth     Authz
}

type listWidgetParams struct {
	Scope           []string  `query:"scope"`
	Q               string    `query:"q"`
	Limit           int       `query:"limit"`
	Offset          int       `query:"offset"`
	IfNoneMatch     string    `header:"If-None-Match"`
	IfModifiedSince time.Time `header:"If-Modified-Since"`
}

type widgetsOut struct {
	ETag         string `header:"ETag"`
	LastModified string `header:"Last-Modified"`
	Body         struct {
		Widgets []widgets.Widget `json:"widgets"`
		Total   int              `json:"total"`
	}
}

type deleteWidgetInput struct {
	ID string `path:"id"`
}

type patchWidgetInput struct {
	ID   string `path:"id"`
	Body WidgetPatch
}

type WidgetPatch struct {
	Enabled     *bool   `json:"enabled,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type widgetOut struct{ Body widgetItem }

type widgetItem struct {
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

func RegisterWidget(api humago.API, h *WidgetHandler) {
	humago.Register(api, humago.Operation{
		OperationID: "listWidgets",
		Method:      http.MethodGet,
		Path:        "/v1/metadata/widgets",
		Summary:     "List widgets",
		Tags:        []string{"Metadata"},
	}, h.list)

	humago.Register(api, humago.Operation{
		OperationID:   "DeleteWidget",
		Method:        http.MethodDelete,
		Path:          "/v1/metadata/widgets/{id}",
		Summary:       "Delete widget",
		Tags:          []string{"Metadata"},
		DefaultStatus: http.StatusNoContent,
	}, h.delete)

	humago.Register(api, humago.Operation{
		OperationID: "PatchWidget",
		Method:      http.MethodPatch,
		Path:        "/v1/metadata/widgets/{id}",
		Summary:     "Patch widget",
		Tags:        []string{"Metadata"},
	}, h.patch)
}

func (h *WidgetHandler) list(ctx context.Context, p *listWidgetParams) (*widgetsOut, error) {
	tenantID := tenant.FromContext(ctx)
	user := middleware.UserFromContext(ctx)
	logger.L.Info("widgets list", "tenant", tenantID, "user", user)

	if p.Offset < 0 {
		p.Offset = 0
	}
	p.Limit = util.SanitizeLimit(p.Limit)

	// Filter out empty scope values to avoid unintended SQL filters like
	// "tenant_scope IN ('')" when the query parameter is omitted.
	var scopes []string
	for _, s := range p.Scope {
		if s != "" {
			scopes = append(scopes, s)
		}
	}

	checkNotModified := func(etag string, last time.Time) error {
		lastStr := last.UTC().Format(http.TimeFormat)
		if p.IfNoneMatch != "" && p.IfNoneMatch == etag {
			hdr := http.Header{}
			hdr.Set("ETag", etag)
			hdr.Set("Last-Modified", lastStr)
			return humago.ErrorWithHeaders(humago.NewError(http.StatusNotModified, ""), hdr)
		}
		if !p.IfModifiedSince.IsZero() && !last.After(p.IfModifiedSince) {
			hdr := http.Header{}
			hdr.Set("ETag", etag)
			hdr.Set("Last-Modified", lastStr)
			return humago.ErrorWithHeaders(humago.NewError(http.StatusNotModified, ""), hdr)
		}
		return nil
	}

	var (
		items []widgets.Widget
		total int
		etag  string
		last  time.Time
		err   error
	)

	if h.Repo != nil {
		f := widgetsrepo.Filter{Tenant: tenantID, ScopeIn: scopes, Q: p.Q, Limit: p.Limit, Offset: p.Offset}
		etag, last, err = h.Repo.GetETagAndLastMod(ctx, f)
		if err != nil {
			return nil, err
		}
		if err := checkNotModified(etag, last); err != nil {
			return nil, err
		}
		var rows []widgetsrepo.Row
		rows, total, err = h.Repo.List(ctx, f)
		if err != nil {
			return nil, err
		}
		items = make([]widgets.Widget, len(rows))
		for i, r := range rows {
			items[i] = widgets.Widget{
				ID:           r.ID,
				Name:         r.Name,
				Version:      r.Version,
				Type:         r.Type,
				Scopes:       r.Scopes,
				Enabled:      r.Enabled,
				Description:  util.Deref(r.Description),
				Capabilities: r.Capabilities,
				Homepage:     util.Deref(r.Homepage),
				UpdatedAt:    r.UpdatedAt,
				Meta:         r.Meta,
				Tenants:      r.Tenants,
			}
		}
	} else {
		items, total, etag, last, err = h.Reg.List(ctx, widgets.Options{
			Scope:  scopes,
			Tenant: tenantID,
			Q:      p.Q,
			Limit:  p.Limit,
			Offset: p.Offset,
		})
		if err != nil {
			return nil, err
		}
		if err := checkNotModified(etag, last); err != nil {
			return nil, err
		}
	}

	out := &widgetsOut{ETag: etag, LastModified: last.UTC().Format(http.TimeFormat)}
	out.Body.Widgets = items
	out.Body.Total = total
	return out, nil
}

func (h *WidgetHandler) delete(ctx context.Context, in *deleteWidgetInput) (*struct{}, error) {
	if h.Auth != nil && !(h.Auth.HasCapability(ctx, "plugins:write") || h.Auth.HasCapability(ctx, "widgets:write")) {
		return nil, humago.NewError(http.StatusForbidden, "forbidden")
	}
	if h.Repo == nil {
		return nil, humago.NewError(http.StatusNotImplemented, "repository not configured")
	}
	if err := h.Repo.Remove(ctx, in.ID); err != nil {
		return nil, humago.Error404NotFound("not found")
	}
	if h.Notifier != nil {
		_ = h.Notifier.NotifyWidgetRemoved(ctx, in.ID)
	}
	return &struct{}{}, nil
}

func (h *WidgetHandler) patch(ctx context.Context, in *patchWidgetInput) (*widgetOut, error) {
	if h.Auth != nil && !(h.Auth.HasCapability(ctx, "plugins:write") || h.Auth.HasCapability(ctx, "widgets:write")) {
		return nil, humago.NewError(http.StatusForbidden, "forbidden")
	}
	if h.Repo == nil {
		return nil, humago.NewError(http.StatusNotImplemented, "repository not configured")
	}
	row, err := h.Repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, humago.Error404NotFound("not found")
	}
	if in.Body.Enabled != nil {
		row.Enabled = *in.Body.Enabled
	}
	if in.Body.Name != nil {
		row.Name = *in.Body.Name
	}
	if in.Body.Description != nil {
		row.Description = in.Body.Description
	}
	row.UpdatedAt = time.Now().UTC()
	if err := h.Repo.Upsert(ctx, row); err != nil {
		return nil, err
	}
	if h.Notifier != nil {
		_ = h.Notifier.NotifyWidgetChanged(ctx, row.ID)
	}
	out := &widgetOut{}
	out.Body = toWidgetItem(row)
	return out, nil
}

func toWidgetItem(r widgetsrepo.Row) widgetItem {
	return widgetItem{
		ID:           r.ID,
		Name:         r.Name,
		Version:      r.Version,
		Type:         r.Type,
		Scopes:       r.Scopes,
		Enabled:      r.Enabled,
		Description:  r.Description,
		Capabilities: r.Capabilities,
		Homepage:     r.Homepage,
		Meta:         r.Meta,
		TenantScope:  r.TenantScope,
		Tenants:      r.Tenants,
		UpdatedAt:    r.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *WidgetHandler) Stream(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	user := middleware.UserFromContext(r.Context())
	logger.L.Info("widgets stream", "tenant", tenantID, "user", user)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}

	ch, unsub := h.Reg.Subscribe()
	defer unsub()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
				logger.L.Error("sse keepalive failed", "error", err)
				return
			}
			flusher.Flush()
		case ev := <-ch:
			if _, err := fmt.Fprintf(w, "event: %s\n", ev.Type); err != nil {
				logger.L.Error("sse write failed", "error", err)
				return
			}
			var (
				b   []byte
				err error
			)
			if ev.Item != nil {
				b, err = json.Marshal(ev.Item)
			} else if ev.ID != "" {
				b, err = json.Marshal(map[string]string{"id": ev.ID})
			} else {
				b = []byte("{}")
			}
			if err != nil {
				logger.L.Error("sse marshal failed", "error", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				logger.L.Error("sse write failed", "error", err)
				return
			}
			flusher.Flush()
		}
	}
}
