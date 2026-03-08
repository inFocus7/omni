package widgets

import (
	"context"
	"html/template"
	"strings"
	"sync"
)

// SizeOption describes one layout size a widget supports.
type SizeOption struct {
	Name string // plugin-chosen label, e.g. "small", "wide", "full"
	W    int    // columns spanned (1-3)
	H    int    // rows spanned (1+)
}

// WidgetDef is the static metadata for a widget.
type WidgetDef struct {
	ID          string
	PluginID    string
	Name        string
	Description string
	Sizes       []SizeOption
}

// Widget is implemented by each dashboard component a plugin provides.
type Widget interface {
	Definition() WidgetDef
	Render(ctx context.Context, filter string, sizeName string) (template.HTML, error)
}

// Registry holds all registered widgets.
type Registry struct {
	mu      sync.RWMutex
	widgets map[string]Widget
}

// NewRegistry creates an empty widget registry.
func NewRegistry() *Registry {
	return &Registry{widgets: make(map[string]Widget)}
}

// Register adds a widget to the registry.
func (r *Registry) Register(w Widget) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.widgets[w.Definition().ID] = w
}

// Get returns a widget by ID.
// Supports instance IDs (e.g. "spacer:0") by stripping the suffix after ":".
func (r *Registry) Get(id string) (Widget, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if w, ok := r.widgets[id]; ok {
		return w, true
	}
	// Try base ID for instance IDs like "spacer:0"
	if base, _, ok := strings.Cut(id, ":"); ok {
		w, found := r.widgets[base]
		return w, found
	}
	return nil, false
}

// All returns definitions of all registered widgets.
func (r *Registry) All() []WidgetDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]WidgetDef, 0, len(r.widgets))
	for _, w := range r.widgets {
		defs = append(defs, w.Definition())
	}
	return defs
}
