package spacer

import (
	"context"
	"html/template"

	"github.com/infocus7/omni/pkg/widgets"
)

// Widget is an invisible spacer that occupies grid cells for layout purposes.
type Widget struct{}

func New() *Widget { return &Widget{} }

func (w *Widget) Definition() widgets.WidgetDef {
	return widgets.WidgetDef{
		ID:       "spacer",
		PluginID: "spacer",
		Name:     "Spacer",
		Sizes: []widgets.SizeOption{
			{Name: "1x1", W: 1, H: 1},
			{Name: "2x1", W: 2, H: 1},
			{Name: "1x2", W: 1, H: 2},
		},
	}
}

func (w *Widget) Render(_ context.Context, _ string, _ string) (template.HTML, error) {
	return "", nil
}
