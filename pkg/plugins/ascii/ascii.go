package ascii

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"sort"
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

// Animation holds parsed metadata and the first frame for server-side rendering.
type Animation struct {
	Name       string
	Size       string
	Cols       int
	Rows       int
	FPS        int
	Palette    map[string]string // class name → CSS colour
	FirstFrame template.HTML
}

// Widget is a dashboard widget that renders an ASCII animation.
// It holds all size variants so Definition() returns every available size
// and Render() dispatches to the correct variant by sizeName.
type Widget struct {
	name        string // UUID — used for widget ID
	displayName string // human-readable name
	packName    string // pack label for grouping in picker
	packAuthor  string // pack author shown as subtitle
	variants    map[string]Animation // size → Animation
	gz          map[string][]byte    // size → gzip-compressed frames blob
}

// AnimationName returns the animation name.
func (w *Widget) AnimationName() string { return w.name }

// FramesGzip returns the gzip-compressed frames for an arbitrary variant.
// Prefer the frames cache / store for size-specific access.
func (w *Widget) FramesGzip() []byte {
	for _, gz := range w.gz {
		return gz
	}
	return nil
}

// asciiTemplateData is the data passed to ascii.tmpl.
type asciiTemplateData struct {
	Scope      string        // container ID: "asc-{name}-{size}"
	Cols       int
	Rows       int
	FPS        int
	PaletteCSS template.HTML // pre-built <style> block, bypasses CSS escaping
	Frame0     template.HTML
	FramesURL  string
}

func (w *Widget) Definition() widgets.WidgetDef {
	sizes := make([]widgets.SizeOption, 0, len(w.variants))
	for _, a := range w.variants {
		sizes = append(sizes, parseSize(a.Size))
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i].Name < sizes[j].Name })
	name := w.displayName
	if name == "" {
		name = w.name
	}
	return widgets.WidgetDef{
		ID:          "ascii-" + w.name,
		PluginID:    "ascii",
		Name:        name,
		Description: "ASCII animation",
		Group:       w.packName,
		GroupAuthor: w.packAuthor,
		Sizes:       sizes,
	}
}

func (w *Widget) Render(_ context.Context, _ string, sizeName string) (template.HTML, error) {
	a, ok := w.variants[sizeName]
	if !ok {
		// Fallback to first available variant if the requested size isn't found.
		for _, v := range w.variants {
			a = v
			break
		}
	}
	scope := "asc-" + a.Name + "-" + a.Size
	data := asciiTemplateData{
		Scope:      scope,
		Cols:       a.Cols,
		Rows:       a.Rows,
		FPS:        a.FPS,
		PaletteCSS: buildPaletteCSS(scope, a.Palette),
		Frame0:     a.FirstFrame,
		FramesURL:  "/api/ascii/frames/" + a.Name + "?size=" + a.Size,
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "ascii.tmpl", data); err != nil {
		return "", fmt.Errorf("render ascii.tmpl: %w", err)
	}
	return template.HTML(buf.String()), nil
}

// buildPaletteCSS constructs a scoped <style> block mapping each named class to its colour.
func buildPaletteCSS(scope string, palette map[string]string) template.HTML {
	if len(palette) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<style>")
	for class, color := range palette {
		fmt.Fprintf(&b, "#%s .%s{color:%s}", scope, template.HTMLEscapeString(class), template.HTMLEscapeString(color))
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

// NewWidgetFromVariants creates a Widget from a slice of store.AnimationVariants.
// All variants must share the same animation. The resulting widget exposes every
// size in Definition().Sizes and dispatches Render() to the correct variant.
func NewWidgetFromVariants(variants []store.AnimationVariant) *Widget {
	if len(variants) == 0 {
		return nil
	}
	id := variants[0].ID
	if id == "" {
		id = variants[0].Name
	}
	w := &Widget{
		name:        id,
		displayName: variants[0].Name,
		variants:    make(map[string]Animation, len(variants)),
		gz:          make(map[string][]byte, len(variants)),
	}
	for _, v := range variants {
		varID := v.ID
		if varID == "" {
			varID = v.Name
		}
		w.variants[v.Size] = Animation{
			Name:       varID,
			Size:       v.Size,
			Cols:       v.Cols,
			Rows:       v.Rows,
			FPS:        v.FPS,
			Palette:    v.Palette,
			FirstFrame: template.HTML(v.FirstFrame),
		}
		w.gz[v.Size] = v.FramesGzip
	}
	return w
}

// NewWidgetFromVariant creates a single-variant Widget.
func NewWidgetFromVariant(v store.AnimationVariant) *Widget {
	return NewWidgetFromVariants([]store.AnimationVariant{v})
}

// LoadFromStore loads all animations from s and returns one Widget per animation
// containing all its size variants.
func LoadFromStore(ctx context.Context, s store.Store) ([]*Widget, error) {
	anims, err := s.ListAnimationsV2(ctx, store.AnimationFilters{})
	if err != nil {
		return nil, fmt.Errorf("list animations: %w", err)
	}
	var result []*Widget
	for _, anim := range anims {
		latestVer, err := s.GetLatestVersion(ctx, anim.ID)
		if err != nil {
			continue
		}
		svs, err := s.ListSizeVariants(ctx, latestVer.ID)
		if err != nil {
			continue
		}
		variants := make([]store.AnimationVariant, 0, len(svs))
		for _, sv := range svs {
			full, err := s.GetSizeVariantByVersionID(ctx, latestVer.ID, sv.Size)
			if err != nil {
				continue
			}
			variants = append(variants, store.AnimationVariant{
				ID:         anim.ID,
				Name:       anim.Name,
				Source:     anim.Source,
				Size:       sv.Size,
				Cols:       sv.Cols,
				Rows:       sv.Rows,
				FPS:        sv.FPS,
				Palette:    full.Palette,
				FirstFrame: full.FirstFrame,
				FramesGzip: full.FramesGzip,
			})
		}
		if w := NewWidgetFromVariants(variants); w != nil {
			w.packName = anim.PackName
			if anim.PackID != "" {
				w.packAuthor = anim.Author
			}
			result = append(result, w)
		}
	}
	return result, nil
}
