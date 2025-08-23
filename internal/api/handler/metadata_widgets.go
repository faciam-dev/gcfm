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

type WidgetHandler struct {
	Reg  widgets.Registry
	Repo widgetsrepo.Repo
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

func RegisterWidget(api humago.API, h *WidgetHandler) {
	humago.Register(api, humago.Operation{
		OperationID: "listWidgets",
		Method:      http.MethodGet,
		Path:        "/v1/metadata/widgets",
		Summary:     "List widgets",
		Tags:        []string{"Metadata"},
	}, h.list)
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
