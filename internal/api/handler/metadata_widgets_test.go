package handler

import (
	"context"
	"testing"
	"time"

	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
)

func TestWidgetHandlerListFallback(t *testing.T) {
	reg := widgetreg.NewInMemory()
	w := widgetreg.Widget{
		ID:        "text-input",
		Name:      "Text Input",
		Version:   "1.0.0",
		Type:      "widget",
		Scopes:    []string{"system"},
		Enabled:   true,
		UpdatedAt: time.Now().UTC(),
	}
	if err := reg.Upsert(context.Background(), w); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	h := &WidgetHandler{Reg: reg}
	out, err := h.list(context.Background(), &listWidgetParams{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if out.Body.Total != 1 || len(out.Body.Widgets) != 1 {
		t.Fatalf("expected 1 widget, got %d", len(out.Body.Widgets))
	}
}
