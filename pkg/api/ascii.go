package api

import (
	"context"
	"net/http"
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
	r.GET("/api/ascii/animations", h.List)
	r.GET("/api/ascii/animations/:name", h.Get)
	r.POST("/api/ascii/animations", h.Create)
	r.PUT("/api/ascii/animations/:name", h.Update)
	r.DELETE("/api/ascii/animations/:name", h.Delete)
	r.POST("/api/ascii/animations/preview", h.Preview)
}

// animationRequest is the HTTP request body for Create and Update.
// Frames accepts plain []string from the client; the handler compresses them.
type animationRequest struct {
	store.AnimationVariant
	Frames []string `json:"frames"`
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
	gz, first, err := store.CompressFrames(req.Frames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	req.AnimationVariant.FramesGzip = gz
	if err := h.putAndRegister(c.Request.Context(), req.AnimationVariant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	gz, first, err := store.CompressFrames(req.Frames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.AnimationVariant.FirstFrame = first
	req.AnimationVariant.FramesGzip = gz
	if err := h.putAndRegister(c.Request.Context(), req.AnimationVariant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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

// putAndRegister stores a variant and eagerly updates the registry and cache.
func (h *AsciiAPI) putAndRegister(ctx context.Context, variant store.AnimationVariant) error {
	if err := h.store.Put(ctx, variant); err != nil {
		return err
	}
	h.registry.Register(asciiplugin.NewWidgetFromVariant(variant))
	h.cache.Store(variant.Name+"/"+variant.Size, variant.FramesGzip)
	return nil
}
