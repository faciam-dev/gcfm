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
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/tenant"
)

type WidgetHandler struct {
	Reg widgets.Registry
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

	opt := widgets.Options{Scope: p.Scope, Tenant: tenantID, Q: p.Q, Limit: p.Limit, Offset: p.Offset}
	items, total, etag, last, err := h.Reg.List(ctx, opt)
	if err != nil {
		return nil, err
	}
	lastStr := last.UTC().Format(http.TimeFormat)
	if p.IfNoneMatch != "" && p.IfNoneMatch == etag {
		hdr := http.Header{}
		hdr.Set("ETag", etag)
		hdr.Set("Last-Modified", lastStr)
		return nil, humago.ErrorWithHeaders(humago.NewError(http.StatusNotModified, ""), hdr)
	}
	if !p.IfModifiedSince.IsZero() && !last.After(p.IfModifiedSince) {
		hdr := http.Header{}
		hdr.Set("ETag", etag)
		hdr.Set("Last-Modified", lastStr)
		return nil, humago.ErrorWithHeaders(humago.NewError(http.StatusNotModified, ""), hdr)
	}

	out := &widgetsOut{ETag: etag, LastModified: lastStr}
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
