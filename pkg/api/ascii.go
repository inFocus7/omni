package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	asciiplugin "github.com/infocus7/omni/pkg/plugins/ascii"
	"github.com/infocus7/omni/pkg/store"
	"github.com/infocus7/omni/pkg/widgets"
)

// AsciiAPI provides HTTP handlers for runtime ASCII animation CRUD.
type AsciiAPI struct {
	store    store.Store
	registry *widgets.Registry
	cache    *sync.Map // map["name/size"][]byte — gzip-compressed frames blob
}

// NewAsciiAPI creates an AsciiAPI.
func NewAsciiAPI(st store.Store, reg *widgets.Registry, cache *sync.Map) *AsciiAPI {
	return &AsciiAPI{store: st, registry: reg, cache: cache}
}

// RegisterRoutes mounts the ASCII CRUD routes on r.
func (h *AsciiAPI) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/ascii/summaries", h.ListSummaries)
	r.GET("/api/ascii/animations", h.List)
	r.GET("/api/ascii/animations/:name", h.Get)
	r.POST("/api/ascii/animations", h.Create)
	r.PUT("/api/ascii/animations/:name", h.Update)
	r.DELETE("/api/ascii/animations/:name", h.Delete)
	r.DELETE("/api/ascii/animations/:name/:size", h.DeleteVariant)
	r.POST("/api/ascii/animations/preview", h.Preview)
	r.POST("/api/ascii/normalize", h.Normalize)
	r.POST("/api/ascii/would-truncate", h.WouldTruncate)
	r.POST("/api/ascii/replace-char", h.ReplaceChar)
	r.POST("/api/ascii/import", h.Import)
	r.POST("/api/ascii/export", h.Export)
	r.GET("/api/ascii/packs", h.ListPacks)
	r.GET("/api/ascii/packs/:id", h.GetPack)
	r.POST("/api/ascii/packs", h.CreatePack)
	r.PUT("/api/ascii/packs/:id", h.UpdatePack)
	r.DELETE("/api/ascii/packs/:id", h.DeletePack)
	r.GET("/api/ascii/authors", h.ListAuthors)
	r.GET("/api/ascii/tags", h.ListTags)
	r.GET("/api/ascii/animations/:name/versions", h.ListVersions)
}

// ImportResult is the response body for POST /api/ascii/import.
type ImportResult struct {
	Imported  []ImportedAnimation `json:"imported"`
	Skipped   []SkippedAnimation  `json:"skipped,omitempty"`
	Conflicts []string            `json:"conflicts,omitempty"`
}

// ImportedAnimation describes a successfully imported animation.
type ImportedAnimation struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Sizes []string `json:"sizes"`
}

// SkippedAnimation describes an animation that was skipped during import.
type SkippedAnimation struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// exportRequest is the HTTP request body for POST /api/ascii/export.
type exportRequest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	License     string   `json:"license"`
	Animations  []string `json:"animations"`
}

// animationRequest is the HTTP request body for Create and Update.
// Accepts either ICG format (class_table + frames as objects) or legacy HTML
// (frames as []string). Detection is automatic.
type animationRequest struct {
	store.AnimationVariant
	// ICG format fields
	ClassTable []string         `json:"class_table,omitempty"`
	ICGFrames  []store.ICGFrame `json:"icg_frames,omitempty"`
	// Legacy HTML format
	Frames []string `json:"frames,omitempty"`
}

// toICG converts the request body to ICGData, handling both ICG and legacy HTML formats.
func (req *animationRequest) toICG() (*store.ICGData, error) {
	if len(req.ClassTable) > 0 && len(req.ICGFrames) > 0 {
		return &store.ICGData{ClassTable: req.ClassTable, Frames: req.ICGFrames}, nil
	}
	if len(req.Frames) > 0 {
		cols := req.Cols
		rows := req.Rows
		if cols < 1 {
			cols = 80
		}
		if rows < 1 {
			rows = 24
		}
		return store.HTMLFramesToICG(req.Frames, cols, rows)
	}
	return nil, fmt.Errorf("request must include either icg_frames+class_table or frames")
}

// ListSummaries returns animation metadata with first frames for gallery rendering.
func (h *AsciiAPI) ListSummaries(c *gin.Context) {
	summaries, err := h.store.ListSummaries(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summaries)
}

// List returns metadata for all animations.
func (h *AsciiAPI) List(c *gin.Context) {
	metas, err := h.store.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metas)
}

// Get returns all variants for the named animation.
func (h *AsciiAPI) Get(c *gin.Context) {
	name := c.Param("name")
	variants, err := h.store.Get(c.Request.Context(), name)
	if err != nil {
		status := http.StatusInternalServerError
		if err == store.ErrNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, variants)
}

// Create accepts a JSON body with animation metadata and frames, and stores the variant.
func (h *AsciiAPI) Create(c *gin.Context) {
	var req animationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	icg, err := req.toICG()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Cols > 0 && req.Rows > 0 {
		icg = store.NormalizeICG(icg, req.Cols, req.Rows)
	}
	gz, first, err := store.CompressICG(icg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	req.AnimationVariant.FramesGzip = gz
	if _, err := h.putAndRegister(c.Request.Context(), req.AnimationVariant); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrInvalidInput) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "name": req.Name})
}

// Update replaces a variant. The URL param is the animation UUID.
func (h *AsciiAPI) Update(c *gin.Context) {
	id := c.Param("name")
	var req animationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	req.AnimationVariant.ID = id
	icg, err := req.toICG()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Cols > 0 && req.Rows > 0 {
		icg = store.NormalizeICG(icg, req.Cols, req.Rows)
	}
	gz, first, err := store.CompressICG(icg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	req.AnimationVariant.FramesGzip = gz
	if _, err := h.putAndRegister(c.Request.Context(), req.AnimationVariant); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrInvalidInput) {
			status = http.StatusBadRequest
		} else if errors.Is(err, store.ErrReadOnly) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Normalize accepts ICG frames + cols/rows and returns normalized ICG frames.
func (h *AsciiAPI) Normalize(c *gin.Context) {
	var req struct {
		ClassTable []string         `json:"class_table"`
		Frames     []store.ICGFrame `json:"frames"`
		Cols       int              `json:"cols"`
		Rows       int              `json:"rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Cols < 1 || req.Rows < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cols and rows must be positive"})
		return
	}
	if len(req.Frames) == 0 {
		c.JSON(http.StatusOK, gin.H{"class_table": req.ClassTable, "frames": []store.ICGFrame{}})
		return
	}
	icg := &store.ICGData{ClassTable: req.ClassTable, Frames: req.Frames}
	normalized := store.NormalizeICG(icg, req.Cols, req.Rows)
	c.JSON(http.StatusOK, gin.H{"class_table": normalized.ClassTable, "frames": normalized.Frames})
}

// WouldTruncate checks if resizing frames to cols×rows would lose visible content.
func (h *AsciiAPI) WouldTruncate(c *gin.Context) {
	var req struct {
		ClassTable []string         `json:"class_table"`
		Frames     []store.ICGFrame `json:"frames"`
		Cols       int              `json:"cols"`
		Rows       int              `json:"rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Cols < 1 || req.Rows < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cols and rows must be positive"})
		return
	}
	icg := &store.ICGData{ClassTable: req.ClassTable, Frames: req.Frames}
	truncates := store.WouldTruncateICG(icg, req.Cols, req.Rows)
	c.JSON(http.StatusOK, gin.H{"truncates": truncates})
}

// ReplaceChar replaces all occurrences of one character with another across
// the provided frames (or a single frame when frame_index is set).
// Colors are preserved — only the character at each cell changes.
func (h *AsciiAPI) ReplaceChar(c *gin.Context) {
	var req struct {
		From       string           `json:"from"`
		To         string           `json:"to"`
		Frames     []store.ICGFrame `json:"frames"`
		FrameIndex *int             `json:"frame_index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	fromRunes := []rune(req.From)
	if len(fromRunes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from character is required"})
		return
	}
	fromRune := fromRunes[0]
	var toRune rune
	if toRunes := []rune(req.To); len(toRunes) > 0 {
		toRune = toRunes[0]
	} else {
		toRune = ' '
	}

	frames := req.Frames
	total := 0
	for i := range frames {
		if req.FrameIndex != nil && i != *req.FrameIndex {
			continue
		}
		newChars, n := replaceRuneInString(frames[i].Chars, fromRune, toRune)
		frames[i].Chars = newChars
		total += n
	}
	c.JSON(http.StatusOK, gin.H{"frames": frames, "count": total})
}

// replaceRuneInString replaces every occurrence of from with to in s,
// returning the new string and the number of replacements made.
func replaceRuneInString(s string, from, to rune) (string, int) {
	if from == to {
		return s, 0
	}
	var b strings.Builder
	b.Grow(len(s))
	count := 0
	for _, r := range s {
		if r == from {
			b.WriteRune(to)
			count++
		} else {
			b.WriteRune(r)
		}
	}
	return b.String(), count
}

// Delete removes an animation from the store, registry, and cache.
func (h *AsciiAPI) Delete(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Delete(c.Request.Context(), name); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, store.ErrReadOnly) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.registry.Unregister("ascii-" + name)
	prefix := name + "/"
	h.cache.Range(func(k, _ any) bool {
		if strings.HasPrefix(k.(string), prefix) {
			h.cache.Delete(k)
		}
		return true
	})
	c.Status(http.StatusNoContent)
}

// DeleteVariant removes a single size variant of an animation.
// If it was the last variant the widget is unregistered from the registry.
func (h *AsciiAPI) DeleteVariant(c *gin.Context) {
	name := c.Param("name")
	size := c.Param("size")
	if err := h.store.DeleteVariant(c.Request.Context(), name, size); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, store.ErrReadOnly) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	// Evict specific cache entry.
	h.cache.Delete(name + "/" + size)
	// Re-evaluate registry: if no variants remain, unregister; otherwise re-register
	// all remaining variants so the widget picker has the complete Sizes list.
	remaining, _ := h.store.Get(c.Request.Context(), name)
	if len(remaining) == 0 {
		h.registry.Unregister("ascii-" + name)
	} else {
		h.registry.Register(asciiplugin.NewWidgetFromVariants(remaining))
	}
	c.Status(http.StatusNoContent)
}

// Preview renders an animation variant in memory without saving it.
func (h *AsciiAPI) Preview(c *gin.Context) {
	var req animationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	icg, err := req.toICG()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, first, err := store.CompressICG(icg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	w := asciiplugin.NewWidgetFromVariant(req.AnimationVariant)
	html, err := w.Render(context.Background(), "", req.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": string(html)})
}

// Import handles POST /api/ascii/import.
// It accepts a multipart/form-data upload of files named "files", auto-detects
// whether it is a single animation or a pack, and stores the results.
func (h *AsciiAPI) Import(c *gin.Context) {
	overwrite := c.Query("overwrite") == "true"

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse form: " + err.Error()})
		return
	}

	// Build path map: relative path → file content.
	// The browser sends each file with its webkitRelativePath as the FIELD NAME
	// (not the filename), because Go's multipart parser sanitizes filenames by
	// stripping directory components. Using the field name preserves the full path.
	pathMap := map[string][]byte{}
	for relPath, fhs := range c.Request.MultipartForm.File {
		if len(fhs) == 0 {
			continue
		}
		f, err := fhs[0].Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file: " + err.Error()})
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file content: " + err.Error()})
			return
		}
		pathMap[relPath] = data
	}

	// Detect import type by checking for pack.json at depth 1 (e.g. "packname/pack.json").
	isPack := false
	for key := range pathMap {
		base := path.Base(key)
		depth := len(strings.Split(strings.Trim(key, "/"), "/"))
		if base == "pack.json" && depth == 2 {
			isPack = true
			break
		}
	}

	ctx := c.Request.Context()
	var result ImportResult

	if isPack {
		result = h.importPack(ctx, pathMap, overwrite)
	} else {
		result = h.importSingle(ctx, pathMap, overwrite, "")
	}

	if !overwrite && len(result.Conflicts) > 0 {
		c.JSON(http.StatusConflict, result)
		return
	}
	c.JSON(http.StatusOK, result)
}

// importSingle imports a single animation from a path map.
// packID, if non-empty, groups the animation under that pack.
func (h *AsciiAPI) importSingle(ctx context.Context, paths map[string][]byte, overwrite bool, packID string) ImportResult {
	// Find meta.json.
	var metaKey string
	var metaData []byte
	for k, v := range paths {
		if path.Base(k) == "meta.json" {
			metaKey = k
			metaData = v
			break
		}
	}
	if metaKey == "" {
		return ImportResult{Skipped: []SkippedAnimation{{Name: "?", Reason: "no meta.json found in upload"}}}
	}

	meta, err := store.ParseMetaJSON(metaData)
	if err != nil {
		return ImportResult{Skipped: []SkippedAnimation{{Name: "?", Reason: "meta.json: " + err.Error()}}}
	}

	// Collision check.
	if !overwrite {
		var exists bool
		if packID != "" {
			_, err := h.store.GetAnimationInPack(ctx, packID, meta.Name)
			exists = err == nil
		} else {
			_, err := h.store.GetAnimationByName(ctx, "user", meta.Name)
			exists = err == nil
		}
		if exists {
			return ImportResult{Conflicts: []string{meta.Name}}
		}
	}

	// Determine the directory prefix for finding frames files.
	// metaKey is e.g. "logo/meta.json" → prefix is "logo/"
	// or "meta.json" → prefix is ""
	prefix := ""
	if dir := path.Dir(metaKey); dir != "." {
		prefix = dir + "/"
	}

	var sizes []string
	var animID string
	for _, variant := range meta.Variants {
		// Find frames file: look for key matching prefix+variant.FramesFile or just the basename.
		framesKey := prefix + variant.FramesFile
		framesData, ok := paths[framesKey]
		if !ok {
			// Fallback: search by basename.
			for k, v := range paths {
				if path.Base(k) == path.Base(variant.FramesFile) {
					framesData = v
					ok = true
					_ = k
					break
				}
			}
		}
		if !ok {
			return ImportResult{
				Skipped: []SkippedAnimation{{
					Name:   meta.Name,
					Reason: fmt.Sprintf("frames file %q not found", variant.FramesFile),
				}},
			}
		}

		var icg *store.ICGData
		if store.IsICGFormat(framesData) {
			icg, err = store.ParseICGFramesFile(framesData)
			if err != nil {
				return ImportResult{
					Skipped: []SkippedAnimation{{
						Name:   meta.Name,
						Reason: fmt.Sprintf("%s/%s: invalid ICG frames: %s", meta.Name, variant.Size, err.Error()),
					}},
				}
			}
		} else {
			var frames []string
			if err := json.Unmarshal(framesData, &frames); err != nil {
				return ImportResult{
					Skipped: []SkippedAnimation{{
						Name:   meta.Name,
						Reason: fmt.Sprintf("%s/%s: invalid frames JSON", meta.Name, variant.Size),
					}},
				}
			}
			icg, err = store.HTMLFramesToICG(frames, variant.Cols, variant.Rows)
			if err != nil {
				return ImportResult{
					Skipped: []SkippedAnimation{{
						Name:   meta.Name,
						Reason: fmt.Sprintf("%s/%s: convert error: %s", meta.Name, variant.Size, err.Error()),
					}},
				}
			}
		}

		gz, firstFrame, err := store.CompressICG(icg)
		if err != nil {
			return ImportResult{
				Skipped: []SkippedAnimation{{
					Name:   meta.Name,
					Reason: fmt.Sprintf("%s/%s: compress error: %s", meta.Name, variant.Size, err.Error()),
				}},
			}
		}

		v := store.AnimationVariant{
			Name:       meta.Name,
			PackID:     packID,
			Source:     "",
			Size:       variant.Size,
			Cols:       variant.Cols,
			Rows:       variant.Rows,
			FPS:        variant.FPS,
			Palette:    meta.Palette,
			FirstFrame: firstFrame,
			FramesGzip: gz,
		}
		id, err := h.putAndRegister(ctx, v)
		if err != nil {
			return ImportResult{
				Skipped: []SkippedAnimation{{
					Name:   meta.Name,
					Reason: fmt.Sprintf("store error: %s", err.Error()),
				}},
			}
		}
		animID = id
		sizes = append(sizes, variant.Size)
	}

	return ImportResult{
		Imported: []ImportedAnimation{{ID: animID, Name: meta.Name, Sizes: sizes}},
	}
}

// importPack imports a multi-animation pack from a path map.
func (h *AsciiAPI) importPack(ctx context.Context, paths map[string][]byte, overwrite bool) ImportResult {
	// Find pack.json.
	var packKey string
	var packData []byte
	for k, v := range paths {
		if path.Base(k) == "pack.json" {
			packKey = k
			packData = v
			break
		}
	}
	if packKey == "" {
		return ImportResult{Skipped: []SkippedAnimation{{Name: "?", Reason: "no pack.json found"}}}
	}

	pack, err := store.ParsePackJSON(packData)
	if err != nil {
		return ImportResult{Skipped: []SkippedAnimation{{Name: "?", Reason: err.Error()}}}
	}

	// Resolve author: use pack.Author or fall back to "user".
	author := pack.Author
	if author == "" {
		author = "user"
	}

	// Find or create the pack row. Never overwrite a remote pack.
	packID := ""
	existing, _ := h.store.ListPacks(ctx, author)
	for _, ep := range existing {
		if ep.Name == pack.Name {
			if ep.Source != "" {
				return ImportResult{Skipped: []SkippedAnimation{{
					Name:   pack.Name,
					Reason: "pack belongs to a remote source and cannot be overwritten",
				}}}
			}
			packID = ep.ID
			break
		}
	}
	if packID == "" {
		p := store.Pack{
			ID:          store.NewUUID(),
			Author:      author,
			Name:         pack.Name,
			Description: pack.Description,
			License:     pack.License,
			Tags:        []string{},
		}
		if err := h.store.PutPack(ctx, p); err != nil {
			return ImportResult{Skipped: []SkippedAnimation{{Name: pack.Name, Reason: "create pack: " + err.Error()}}}
		}
		packID = p.ID
	}

	// The pack root is the directory containing pack.json (e.g. "my-pack/").
	packPrefix := ""
	if dir := path.Dir(packKey); dir != "." {
		packPrefix = dir + "/"
	}

	var result ImportResult
	for _, animName := range pack.Animations {
		animPrefix := packPrefix + animName + "/"
		subPaths := map[string][]byte{}
		for k, v := range paths {
			if strings.HasPrefix(k, animPrefix) {
				rel := strings.TrimPrefix(k, animPrefix)
				subPaths[rel] = v
			}
		}

		if _, ok := subPaths["meta.json"]; !ok {
			result.Skipped = append(result.Skipped, SkippedAnimation{
				Name:   animName,
				Reason: "meta.json not found",
			})
			continue
		}

		sub := h.importSingle(ctx, subPaths, overwrite, packID)
		result.Imported = append(result.Imported, sub.Imported...)
		result.Skipped = append(result.Skipped, sub.Skipped...)
		result.Conflicts = append(result.Conflicts, sub.Conflicts...)
	}

	return result
}

// Export handles POST /api/ascii/export.
// It builds a zip of the requested local animations and writes it to the response.
func (h *AsciiAPI) Export(c *gin.Context) {
	var req exportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if len(req.Animations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "animations list must not be empty"})
		return
	}

	ctx := c.Request.Context()

	// Fetch all animations and validate they are local.
	type animData struct {
		name     string // slug, used for zip paths
		variants []store.AnimationVariant
	}
	anims := make([]animData, 0, len(req.Animations))

	for _, id := range req.Animations {
		anim, err := h.store.GetAnimationByID(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("animation %q not found", id)})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		variants, err := h.store.Get(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, v := range variants {
			if v.Source != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s is a remote animation and cannot be exported", anim.Name)})
				return
			}
		}
		anims = append(anims, animData{name: anim.Name, variants: variants})
	}

	// Build zip in memory.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	isPack := len(anims) >= 2
	packPrefix := ""
	if isPack {
		packName := req.Name
		if packName == "" {
			packName = "export"
		}
		packPrefix = packName + "/"

		// Write pack.json.
		animNames := make([]string, len(anims))
		for i, a := range anims {
			animNames[i] = a.name
		}
		packJSON := store.PackJSON{
			Name:        req.Name,
			Version:     req.Version,
			Author:      req.Author,
			Description: req.Description,
			License:     req.License,
			Animations:  animNames,
		}
		packJSONBytes, err := json.Marshal(packJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal pack.json: " + err.Error()})
			return
		}
		if err := writeZipFile(zw, packPrefix+"pack.json", packJSONBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	for _, anim := range anims {
		// Build the directory prefix for this animation.
		animDir := packPrefix + anim.name + "/"

		// Reconstruct PackMeta from variant data.
		var palette map[string]string
		variants := make([]store.VariantFileMeta, 0, len(anim.variants))
		for _, v := range anim.variants {
			if palette == nil && v.Palette != nil {
				palette = v.Palette
			}
			framesFile := fmt.Sprintf("frames-%s.json", v.Size)
			variants = append(variants, store.VariantFileMeta{
				Size:       v.Size,
				Cols:       v.Cols,
				Rows:       v.Rows,
				FPS:        v.FPS,
				FramesFile: framesFile,
			})
		}

		meta := store.PackMeta{
			Name:     anim.name,
			Palette:  palette,
			Variants: variants,
		}
		metaBytes, err := json.Marshal(meta)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal meta.json: " + err.Error()})
			return
		}
		if err := writeZipFile(zw, animDir+"meta.json", metaBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Write frames files (ICG format).
		for _, v := range anim.variants {
			icg, err := store.DecompressICG(v.FramesGzip)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("decompress frames for %s/%s: %s", anim.name, v.Size, err.Error())})
				return
			}
			icgJSON, err := json.Marshal(icg)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("marshal ICG for %s/%s: %s", anim.name, v.Size, err.Error())})
				return
			}
			framesFile := fmt.Sprintf("frames-%s.json", v.Size)
			if err := writeZipFile(zw, animDir+framesFile, icgJSON); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}

	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "close zip: " + err.Error()})
		return
	}

	zipName := req.Name
	if zipName == "" {
		if len(anims) == 1 {
			zipName = anims[0].name
		} else {
			zipName = "export"
		}
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, zipName))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// writeZipFile writes data into a zip archive under the given path.
func writeZipFile(zw *zip.Writer, zipPath string, data []byte) error {
	w, err := zw.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip entry %q: %w", zipPath, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write zip entry %q: %w", zipPath, err)
	}
	return nil
}

// putAndRegister stores a variant and eagerly updates the registry and cache.
// Returns the animation UUID.
func (h *AsciiAPI) putAndRegister(ctx context.Context, variant store.AnimationVariant) (string, error) {
	id, err := h.store.Put(ctx, variant)
	if err != nil {
		return "", err
	}
	if all, err := h.store.Get(ctx, id); err == nil && len(all) > 0 {
		h.registry.Register(asciiplugin.NewWidgetFromVariants(all))
		h.cache.Delete(id + "/" + variant.Size)
	}
	return id, nil
}

// ListPacks returns all packs, optionally filtered by author.
func (h *AsciiAPI) ListPacks(c *gin.Context) {
	packs, err := h.store.ListPacks(c.Request.Context(), c.Query("author"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, packs)
}

// GetPack returns a single pack by ID.
func (h *AsciiAPI) GetPack(c *gin.Context) {
	p, err := h.store.GetPack(c.Request.Context(), c.Param("id"))
	if err != nil {
		status := http.StatusInternalServerError
		if err == store.ErrNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

// CreatePack creates a new pack.
func (h *AsciiAPI) CreatePack(c *gin.Context) {
	var p store.Pack
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if p.Author == "" || p.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "author and slug are required"})
		return
	}
	if err := h.store.PutPack(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

// UpdatePack updates an existing pack.
func (h *AsciiAPI) UpdatePack(c *gin.Context) {
	var p store.Pack
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	p.ID = c.Param("id")
	if err := h.store.PutPack(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeletePack removes a pack by ID.
func (h *AsciiAPI) DeletePack(c *gin.Context) {
	if err := h.store.DeletePack(c.Request.Context(), c.Param("id")); err != nil {
		status := http.StatusInternalServerError
		if err == store.ErrNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListAuthors returns all distinct animation authors.
func (h *AsciiAPI) ListAuthors(c *gin.Context) {
	authors, err := h.store.ListAuthors(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authors)
}

// ListTags returns all distinct animation tags.
func (h *AsciiAPI) ListTags(c *gin.Context) {
	tags, err := h.store.ListDistinctTagsV2(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// ListVersions returns all versions for an animation.
func (h *AsciiAPI) ListVersions(c *gin.Context) {
	versions, err := h.store.ListVersions(c.Request.Context(), c.Param("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, versions)
}
