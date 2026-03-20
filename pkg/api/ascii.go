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
	r.POST("/api/ascii/import", h.Import)
	r.POST("/api/ascii/export", h.Export)
}

// ImportResult is the response body for POST /api/ascii/import.
type ImportResult struct {
	Imported  []ImportedAnimation `json:"imported"`
	Skipped   []SkippedAnimation  `json:"skipped,omitempty"`
	Conflicts []string            `json:"conflicts,omitempty"`
}

// ImportedAnimation describes a successfully imported animation.
type ImportedAnimation struct {
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
// Frames accepts plain []string from the client; the handler compresses them.
type animationRequest struct {
	store.AnimationVariant
	Frames []string `json:"frames"`
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
	if req.Cols > 0 && req.Rows > 0 {
		normalized, err := store.NormalizeFrames(req.Frames, req.Cols, req.Rows)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "normalize: " + err.Error()})
			return
		}
		req.Frames = normalized
	}
	gz, first, err := store.CompressFrames(req.Frames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	req.AnimationVariant.FramesGzip = gz
	if err := h.putAndRegister(c.Request.Context(), req.AnimationVariant); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrInvalidInput) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "name": req.Name})
}

// Update replaces a variant (name in URL must match body).
func (h *AsciiAPI) Update(c *gin.Context) {
	name := c.Param("name")
	var req animationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Name == "" {
		req.Name = name
	}
	if req.Cols > 0 && req.Rows > 0 {
		normalized, err := store.NormalizeFrames(req.Frames, req.Cols, req.Rows)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "normalize: " + err.Error()})
			return
		}
		req.Frames = normalized
	}
	gz, first, err := store.CompressFrames(req.Frames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	req.AnimationVariant.FramesGzip = gz
	if err := h.putAndRegister(c.Request.Context(), req.AnimationVariant); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrInvalidInput) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Normalize accepts frames + cols/rows and returns frames normalized to exact dimensions.
// This offloads the expensive HTML parse → grid → serialize round-trip from the client.
func (h *AsciiAPI) Normalize(c *gin.Context) {
	var req struct {
		Frames []string `json:"frames"`
		Cols   int      `json:"cols"`
		Rows   int      `json:"rows"`
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
		c.JSON(http.StatusOK, gin.H{"frames": []string{}})
		return
	}
	normalized, err := store.NormalizeFrames(req.Frames, req.Cols, req.Rows)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"frames": normalized})
}

// Delete removes an animation from the store, registry, and cache.
func (h *AsciiAPI) Delete(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Delete(c.Request.Context(), name); err != nil {
		status := http.StatusInternalServerError
		if err == store.ErrNotFound {
			status = http.StatusNotFound
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
		if err == store.ErrNotFound {
			status = http.StatusNotFound
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
	_, first, err := store.CompressFrames(req.Frames)
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
		result = h.importSingle(ctx, pathMap, overwrite)
	}

	if !overwrite && len(result.Conflicts) > 0 {
		c.JSON(http.StatusConflict, result)
		return
	}
	c.JSON(http.StatusOK, result)
}

// importSingle imports a single animation from a path map.
// The paths may be at root level ("meta.json") or one folder deep ("animname/meta.json").
func (h *AsciiAPI) importSingle(ctx context.Context, paths map[string][]byte, overwrite bool) ImportResult {
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
		if _, err := h.store.Get(ctx, meta.Name); err == nil {
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

		var frames []string
		if err := json.Unmarshal(framesData, &frames); err != nil {
			return ImportResult{
				Skipped: []SkippedAnimation{{
					Name:   meta.Name,
					Reason: fmt.Sprintf("%s/%s: invalid frames JSON", meta.Name, variant.Size),
				}},
			}
		}

		gz, firstFrame, err := store.CompressFrames(frames)
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
			Source:     "", // always local on import
			Size:       variant.Size,
			Cols:       variant.Cols,
			Rows:       variant.Rows,
			FPS:        variant.FPS,
			Palette:    meta.Palette,
			FirstFrame: firstFrame,
			FramesGzip: gz,
		}
		if err := h.putAndRegister(ctx, v); err != nil {
			return ImportResult{
				Skipped: []SkippedAnimation{{
					Name:   meta.Name,
					Reason: fmt.Sprintf("store error: %s", err.Error()),
				}},
			}
		}
		sizes = append(sizes, variant.Size)
	}

	return ImportResult{
		Imported: []ImportedAnimation{{Name: meta.Name, Sizes: sizes}},
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

	// The pack root is the directory containing pack.json (e.g. "my-pack/").
	packPrefix := ""
	if dir := path.Dir(packKey); dir != "." {
		packPrefix = dir + "/"
	}

	var result ImportResult
	for _, animName := range pack.Animations {
		// Filter paths to those starting with packPrefix+animName+"/".
		animPrefix := packPrefix + animName + "/"
		subPaths := map[string][]byte{}
		for k, v := range paths {
			if strings.HasPrefix(k, animPrefix) {
				// Strip the animPrefix so importSingle sees "meta.json", "frames-1x1.json", etc.
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

		sub := h.importSingle(ctx, subPaths, overwrite)
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
		name     string
		variants []store.AnimationVariant
	}
	anims := make([]animData, 0, len(req.Animations))

	for _, name := range req.Animations {
		variants, err := h.store.Get(ctx, name)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("animation %q not found", name)})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		for _, v := range variants {
			if v.Source != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s is a remote animation and cannot be exported", name)})
				return
			}
		}
		anims = append(anims, animData{name: name, variants: variants})
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

		// Write frames files.
		for _, v := range anim.variants {
			rawJSON, err := store.GzipDecompress(v.FramesGzip)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("decompress frames for %s/%s: %s", anim.name, v.Size, err.Error())})
				return
			}
			framesFile := fmt.Sprintf("frames-%s.json", v.Size)
			if err := writeZipFile(zw, animDir+framesFile, rawJSON); err != nil {
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
// The cache entry is invalidated (not eagerly written) so the Watcher goroutine
// repopulates it with the sanitized frames broadcast via the store event.
func (h *AsciiAPI) putAndRegister(ctx context.Context, variant store.AnimationVariant) error {
	if err := h.store.Put(ctx, variant); err != nil {
		return err
	}
	// Re-register all variants so Definition().Sizes stays complete.
	if all, err := h.store.Get(ctx, variant.Name); err == nil && len(all) > 0 {
		h.registry.Register(asciiplugin.NewWidgetFromVariants(all))
	}
	h.cache.Delete(variant.Name + "/" + variant.Size)
	return nil
}
