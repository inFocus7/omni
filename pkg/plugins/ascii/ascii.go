package ascii

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"strconv"
	"strings"

	"github.com/infocus7/omni/pkg/store"
	"github.com/infocus7/omni/pkg/widgets"
)

//go:embed data
var dataFS embed.FS

//go:embed templates/*.tmpl
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// EmbeddedDataFS returns the embedded animation data filesystem (the "data/" subtree).
// Use this to construct a store.EmbeddedStore backed by the built-in animations.
func EmbeddedDataFS() (fs.FS, error) {
	return fs.Sub(dataFS, "data")
}

// Animation holds parsed metadata and pre-rendered HTML frames.
type Animation struct {
	Name    string
	Size    string
	Cols    int
	Rows    int
	FPS     int
	Palette map[string]string // class name → CSS colour
	Frames  []template.HTML
}

// Widget is a dashboard widget that renders an ASCII animation.
type Widget struct {
	anim       Animation
	framesJSON []byte // pre-serialized []string JSON for the frames API
}

// FramesJSON returns the pre-serialized JSON array of frame HTML strings.
func (w *Widget) FramesJSON() []byte { return w.framesJSON }

// AnimationName returns the animation name.
func (w *Widget) AnimationName() string { return w.anim.Name }

// asciiTemplateData is the data passed to ascii.tmpl.
type asciiTemplateData struct {
	Cols       int
	Rows       int
	FPS        int
	PaletteCSS template.HTML // pre-built <style> block, bypasses CSS escaping
	Frame0     template.HTML
	FramesURL  string
}

func (w *Widget) Definition() widgets.WidgetDef {
	size := parseSize(w.anim.Size)
	return widgets.WidgetDef{
		ID:          "ascii-" + w.anim.Name,
		PluginID:    "ascii",
		Name:        w.anim.Name,
		Description: "ASCII animation",
		Sizes:       []widgets.SizeOption{size},
	}
}

func (w *Widget) Render(_ context.Context, _ string, _ string) (template.HTML, error) {
	a := w.anim
	var frame0 template.HTML
	if len(a.Frames) > 0 {
		frame0 = a.Frames[0]
	}
	data := asciiTemplateData{
		Cols:       a.Cols,
		Rows:       a.Rows,
		FPS:        a.FPS,
		PaletteCSS: buildPaletteCSS(a.Palette),
		Frame0:     frame0,
		FramesURL:  "/api/ascii/" + a.Name + "/frames",
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "ascii.tmpl", data); err != nil {
		return "", fmt.Errorf("render ascii.tmpl: %w", err)
	}
	return template.HTML(buf.String()), nil
}

// buildPaletteCSS constructs a <style> block mapping each named class to its colour.
// Returns template.HTML to bypass html/template's CSS-context sanitization,
// which would otherwise replace unrecognized CSS values with "ZgotmplZ".
func buildPaletteCSS(palette map[string]string) template.HTML {
	if len(palette) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<style>")
	for class, color := range palette {
		fmt.Fprintf(&b, ".%s{color:%s}", template.HTMLEscapeString(class), template.HTMLEscapeString(color))
	}
	b.WriteString("</style>")
	return template.HTML(b.String())
}

// parseSize converts "2x1" into a SizeOption{Name:"2x1", W:2, H:1}.
func parseSize(s string) widgets.SizeOption {
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return widgets.SizeOption{Name: s, W: 1, H: 1}
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return widgets.SizeOption{Name: s, W: w, H: h}
}

// NewWidgetFromVariant creates a Widget from a store.AnimationVariant.
func NewWidgetFromVariant(v store.AnimationVariant) (*Widget, error) {
	frames := make([]template.HTML, len(v.Frames))
	for i, f := range v.Frames {
		frames[i] = template.HTML(f) // pre-rendered, trusted
	}
	a := Animation{
		Name:    v.Name,
		Size:    v.Size,
		Cols:    v.Cols,
		Rows:    v.Rows,
		FPS:     v.FPS,
		Palette: v.Palette, // map[string]string
		Frames:  frames,
	}
	return newWidget(a)
}

// LoadFromStore loads all animations from s and returns them as Widgets.
func LoadFromStore(ctx context.Context, s store.Store) ([]*Widget, error) {
	metas, err := s.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list animations: %w", err)
	}
	var result []*Widget
	for _, meta := range metas {
		variants, err := s.Get(ctx, meta.Name)
		if err != nil {
			// Skip animations whose frame data isn't ready yet; the Watch
			// goroutine will register them once ConfigMaps are available.
			continue
		}
		for _, v := range variants {
			w, err := NewWidgetFromVariant(v)
			if err != nil {
				continue
			}
			result = append(result, w)
		}
	}
	return result, nil
}

// newWidget creates a Widget with pre-serialized framesJSON.
func newWidget(a Animation) (*Widget, error) {
	strs := make([]string, len(a.Frames))
	for i, f := range a.Frames {
		strs[i] = string(f)
	}
	fj, err := json.Marshal(strs)
	if err != nil {
		return nil, fmt.Errorf("serialize frames for %q: %w", a.Name, err)
	}
	return &Widget{anim: a, framesJSON: fj}, nil
}
