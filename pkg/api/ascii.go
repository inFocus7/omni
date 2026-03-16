package api

import (
	"context"
	"encoding/json"
	"net/http"
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
	cache    *sync.Map // map[string][]byte — animation name → framesJSON
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

// Create accepts a meta.json body and stores the animation.
func (h *AsciiAPI) Create(c *gin.Context) {
	var variant store.AnimationVariant
	if err := c.ShouldBindJSON(&variant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if err := h.putAndRegister(c.Request.Context(), variant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "name": variant.Name})
}

// Update replaces a variant's metadata (name in URL must match body).
func (h *AsciiAPI) Update(c *gin.Context) {
	name := c.Param("name")
	var variant store.AnimationVariant
	if err := c.ShouldBindJSON(&variant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if variant.Name == "" {
		variant.Name = name
	}
	if err := h.putAndRegister(c.Request.Context(), variant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Delete removes an animation from the store and registry.
func (h *AsciiAPI) Delete(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Delete(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Eagerly remove from registry and cache.
	h.registry.Unregister("ascii-" + name)
	h.cache.Delete(name)
	c.Status(http.StatusNoContent)
}

// Preview renders an animation variant in memory without saving it.
func (h *AsciiAPI) Preview(c *gin.Context) {
	var variant store.AnimationVariant
	if err := c.ShouldBindJSON(&variant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	w, err := asciiplugin.NewWidgetFromVariant(variant)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid animation: " + err.Error()})
		return
	}
	html, err := w.Render(context.Background(), "", variant.Size)
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
	w, err := asciiplugin.NewWidgetFromVariant(variant)
	if err != nil {
		return err
	}
	h.registry.Register(w)

	// Serialise frames for the cache.
	strs := make([]string, len(variant.Frames))
	copy(strs, variant.Frames)
	fj, err := json.Marshal(strs)
	if err != nil {
		return err
	}
	h.cache.Store(variant.Name, fj)
	return nil
}
