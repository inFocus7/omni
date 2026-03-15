package ascii

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"embed"

	"github.com/infocus7/omni/pkg/widgets"
)

//go:embed data
var dataFS embed.FS

//go:embed templates/*.tmpl
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// animationFile is the on-disk format of animation.json.
type animationFile struct {
	Name    string   `json:"name"`
	Size    string   `json:"size"`
	Cols    int      `json:"cols"`
	Rows    int      `json:"rows"`
	FPS     int      `json:"fps"`
	Palette []string `json:"palette,omitempty"`
	Frames  []string `json:"frames"`
}

// Animation holds the parsed metadata and pre-rendered HTML frames.
type Animation struct {
	Name    string
	Size    string
	Cols    int
	Rows    int
	FPS     int
	Palette []string
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

// buildPaletteCSS constructs a <style> block mapping .ac0, .ac1, … to colors.
// Returns template.HTML to bypass html/template's CSS-context sanitization,
// which would otherwise replace unrecognized CSS values with "ZgotmplZ".
func buildPaletteCSS(palette []string) template.HTML {
	if len(palette) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<style>")
	for i, color := range palette {
		fmt.Fprintf(&b, ".ac%d{color:%s}", i, template.HTMLEscapeString(color))
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

// LoadAnimations walks the given fs.FS for directories containing animation.json,
// loading all frames from the single packed file.
func LoadAnimations(fsys fs.FS) ([]Animation, error) {
	var anims []Animation

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		anim, err := loadAnimation(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("load animation %q: %w", entry.Name(), err)
		}
		anims = append(anims, anim)
	}
	return anims, nil
}

func loadAnimation(fsys fs.FS, dir string) (Animation, error) {
	animPath := filepath.Join(dir, "meta.json")
	data, err := fs.ReadFile(fsys, animPath)
	if err != nil {
		return Animation{}, fmt.Errorf("read meta.json (new format required): %w", err)
	}

	var af animationFile
	if err := json.Unmarshal(data, &af); err != nil {
		return Animation{}, fmt.Errorf("parse meta.json: %w", err)
	}

	frames := make([]template.HTML, len(af.Frames))
	for i, s := range af.Frames {
		frames[i] = template.HTML(s) // pre-rendered, trusted
	}

	return Animation{
		Name:    af.Name,
		Size:    af.Size,
		Cols:    af.Cols,
		Rows:    af.Rows,
		FPS:     af.FPS,
		Palette: af.Palette,
		Frames:  frames,
	}, nil
}

// LoadAll loads animations from the embedded data FS first, then from
// $OMNI_DATA_DIR/ascii/ if set. External animations override built-ins by name.
func LoadAll() ([]*Widget, error) {
	byName := map[string]*Widget{}

	// Load embedded built-ins from data/ subdirectory.
	sub, err := fs.Sub(dataFS, "data")
	if err != nil {
		return nil, fmt.Errorf("sub embedded data: %w", err)
	}
	builtins, err := LoadAnimations(sub)
	if err != nil {
		return nil, fmt.Errorf("load built-in animations: %w", err)
	}
	for _, a := range builtins {
		w, err := newWidget(a)
		if err != nil {
			return nil, err
		}
		byName[a.Name] = w
	}

	// Load external animations from $OMNI_DATA_DIR/ascii/ if set.
	if dataDir := os.Getenv("OMNI_DATA_DIR"); dataDir != "" {
		asciiDir := filepath.Join(dataDir, "ascii")
		if info, err := os.Stat(asciiDir); err == nil && info.IsDir() {
			extFS := os.DirFS(asciiDir)
			externals, err := LoadAnimations(extFS)
			if err != nil {
				return nil, fmt.Errorf("load external animations: %w", err)
			}
			for _, a := range externals {
				w, err := newWidget(a)
				if err != nil {
					return nil, err
				}
				byName[a.Name] = w
			}
		}
	}

	result := make([]*Widget, 0, len(byName))
	for _, w := range byName {
		result = append(result, w)
	}
	return result, nil
}

// newWidget creates a Widget with pre-serialized framesJSON.
func newWidget(a Animation) (*Widget, error) {
	// Build []string for JSON serialization (raw HTML strings).
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
