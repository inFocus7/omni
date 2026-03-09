package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/infocus7/omni/pkg/plugins"
	"github.com/infocus7/omni/pkg/settings"
	"github.com/infocus7/omni/ui"

	"github.com/gin-gonic/gin"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

// respondError logs the error and sends a JSON error response.
func respondError(c *gin.Context, logger zerolog.Logger, status int, err error, msg string) {
	logger.Error().Err(err).Msg(msg)
	c.JSON(status, gin.H{"error": err.Error()})
}

// containsString checks if a string slice contains a given string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// hasWidgetWithID checks if a widget with the given ID exists in the dashboard widgets.
func hasWidgetWithID(widgets []settings.DashboardWidget, id string) bool {
	for _, w := range widgets {
		if w.ID == id {
			return true
		}
	}
	return false
}

// filterStrings returns a new slice with only strings not matching the given value.
func filterStrings(slice []string, exclude string) []string {
	result := slice[:0]
	for _, s := range slice {
		if s != exclude {
			result = append(result, s)
		}
	}
	return result
}

// filterWidgets returns a new slice with only widgets not matching the given ID.
func filterWidgets(widgets []settings.DashboardWidget, excludeID string) []settings.DashboardWidget {
	result := widgets[:0]
	for _, w := range widgets {
		if w.ID != excludeID {
			result = append(result, w)
		}
	}
	return result
}

func main() {
	ctx := context.Background()
	logger := log.With().Str("component", "app").Logger()

	s, err := settings.Load()
	if err != nil {
		logger.Warn().Err(err).Msg("could not load settings")
		s = &settings.Settings{}
	}

	pm, err := plugins.NewPluginManager(ctx, s)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize plugin manager")
	}

	pages, err := ui.Pages()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load UI pages")
	}

	// render executes a page template by name using the shared base layout.
	render := func(c *gin.Context, page string, data any) error {
		t, ok := pages[page]
		if !ok {
			return fmt.Errorf("unknown page template: %s", page)
		}
		return t.ExecuteTemplate(c.Writer, "base.tmpl", data)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next() // process
		duration := time.Since(start)

		logger := log.With().Str("component", "handler").Logger()
		logger.Info().
			Int("status", c.Writer.Status()).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Dur("latency", duration).
			Msg("handled request")
	})
	r.Static("/static", "./ui/static")

	r.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNotFound)
		if err := render(c, "404.tmpl", nil); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to render 404 template")
		}
	})

	// ── Dashboard ──────────────────────────────────────────
	r.GET("/", func(c *gin.Context) {
		filter := c.DefaultQuery("filter", "7d")

		// Determine active column count (defaults to 5 = desktop)
		cols := 5
		if q := c.Query("cols"); q != "" {
			if n, err := strconv.Atoi(q); err == nil && (n == 5 || n == 3 || n == 2) {
				cols = n
			}
		}

		layoutMode := s.Dashboard.LayoutMode
		widgetList := s.Dashboard.WidgetsForCols(cols)

		// Use widget-based dashboard if user has pinned widgets
		if len(widgetList) > 0 {
			rendered, err := pm.FetchDashboardWidgets(filter, cols)
			if err != nil {
				respondError(c, logger, http.StatusInternalServerError, err, "failed to fetch dashboard widgets")
				return
			}

			data := &plugins.DashboardData{
				ActiveFilter: filter,
				Widgets:      rendered,
				LayoutMode:   layoutMode,
				ActiveCols:   cols,
			}
			if err := render(c, "dashboard.tmpl", data); err != nil {
				respondError(c, logger, http.StatusInternalServerError, err, "failed to render dashboard template")
			}
			return
		}

		// Empty dashboard — show empty state (no widgets fetched)
		data := &plugins.DashboardData{
			ActiveFilter: filter,
			LayoutMode:   layoutMode,
			ActiveCols:   cols,
		}
		if err := render(c, "dashboard.tmpl", data); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to render dashboard template")
		}
	})

	r.GET("/plugins", func(c *gin.Context) {
		if err := render(c, "plugins_page.tmpl", nil); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to render plugins page template")
			return
		}
	})

	r.GET("/github", func(c *gin.Context) {
		filter := c.DefaultQuery("filter", "7d")

		data, err := pm.FetchGitHubDetail(filter)
		if err != nil {
			logger.Error().Err(err).Fields(map[string]interface{}{
				"filter": filter,
			}).Msg("failed to fetch GitHub detail data")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := render(c, "github_page.tmpl", data); err != nil {
			logger.Error().Err(err).Fields(map[string]interface{}{
				"filter": filter,
			}).Msg("failed to render GitHub page template")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	r.GET("/settings", func(c *gin.Context) {
		section := c.DefaultQuery("section", "general")

		data := gin.H{
			"Section":  section,
			"Settings": s,
		}

		if err := render(c, "settings_page.tmpl", data); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to render settings page template")
			return
		}
	})

	r.POST("/settings/github/watch", func(c *gin.Context) {
		raw := c.PostForm("entry")
		entry := settings.NormalizeEntry(raw)
		if entry == "" {
			c.Redirect(http.StatusSeeOther, "/settings?section=plugins")
			return
		}

		// Deduplicate
		if containsString(s.GitHub.Watched, entry) {
			c.Redirect(http.StatusSeeOther, "/settings?section=plugins")
			return
		}

		s.GitHub.Watched = append(s.GitHub.Watched, entry)
		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.Redirect(http.StatusSeeOther, "/settings?section=plugins")
	})

	r.DELETE("/settings/github/watch/:entry", func(c *gin.Context) {
		entry, err := url.PathUnescape(c.Param("entry"))
		if err != nil {
			logger.Error().Err(err).Msg("failed to unescape entry parameter")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entry"})
			return
		}

		s.GitHub.Watched = filterStrings(s.GitHub.Watched, entry)

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.Status(http.StatusNoContent)
	})

	// ── Widget API ─────────────────────────────────────────

	// List all available widgets with their size options
	r.GET("/api/widgets", func(c *gin.Context) {
		defs := pm.Registry.All()

		type sizeJSON struct {
			Name string `json:"name"`
			W    int    `json:"w"`
			H    int    `json:"h"`
		}
		type widgetJSON struct {
			ID          string     `json:"id"`
			PluginID    string     `json:"plugin_id"`
			Name        string     `json:"name"`
			Description string     `json:"description"`
			Sizes       []sizeJSON `json:"sizes"`
		}

		out := make([]widgetJSON, 0, len(defs))
		for _, d := range defs {
			sizes := make([]sizeJSON, len(d.Sizes))
			for j, sz := range d.Sizes {
				sizes[j] = sizeJSON{Name: sz.Name, W: sz.W, H: sz.H}
			}
			out = append(out, widgetJSON{
				ID:          d.ID,
				PluginID:    d.PluginID,
				Name:        d.Name,
				Description: d.Description,
				Sizes:       sizes,
			})
		}
		c.JSON(http.StatusOK, out)
	})

	// Preview a widget at a given size
	r.GET("/api/widgets/:id/preview", func(c *gin.Context) {
		id := c.Param("id")
		sizeName := c.DefaultQuery("size", "")
		filter := c.DefaultQuery("filter", "7d")

		if sizeName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "size query parameter required"})
			return
		}

		html, w, h, err := pm.RenderWidgetPreview(id, sizeName, filter)
		if err != nil {
			logger.Error().Err(err).Str("widget", id).Str("size", sizeName).Msg("failed to render widget preview")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"html": string(html),
			"w":    w,
			"h":    h,
		})
	})

	// Pin a widget to the dashboard
	r.POST("/api/dashboard/widgets", func(c *gin.Context) {
		var req struct {
			ID       string `json:"id"`
			SizeName string `json:"size_name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// Check widget exists
		if _, ok := pm.Registry.Get(req.ID); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown widget"})
			return
		}

		// Deduplicate
		if hasWidgetWithID(s.Dashboard.Widgets, req.ID) {
			c.JSON(http.StatusConflict, gin.H{"error": "widget already pinned"})
			return
		}

		pos := len(s.Dashboard.Widgets)
		s.Dashboard.Widgets = append(s.Dashboard.Widgets, settings.DashboardWidget{
			ID:       req.ID,
			SizeName: req.SizeName,
			Position: pos,
		})

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	// Remove a widget from the dashboard
	r.DELETE("/api/dashboard/widgets/:id", func(c *gin.Context) {
		id := c.Param("id")

		s.Dashboard.Widgets = filterWidgets(s.Dashboard.Widgets, id)
		// Re-index positions
		for i := range s.Dashboard.Widgets {
			s.Dashboard.Widgets[i].Position = i
		}

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.Status(http.StatusNoContent)
	})

	// Resize a widget
	r.PUT("/api/dashboard/widgets/:id/size", func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			SizeName string `json:"size_name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		found := false
		for i, w := range s.Dashboard.Widgets {
			if w.ID == id {
				s.Dashboard.Widgets[i].SizeName = req.SizeName
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not pinned"})
			return
		}

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Reorder widgets
	r.PUT("/api/dashboard/widgets/order", func(c *gin.Context) {
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// Build lookup for current widgets
		lookup := make(map[string]*settings.DashboardWidget)
		for i := range s.Dashboard.Widgets {
			lookup[s.Dashboard.Widgets[i].ID] = &s.Dashboard.Widgets[i]
		}

		// Re-assign positions based on the new order
		for i, id := range req.IDs {
			if w, ok := lookup[id]; ok {
				w.Position = i
			}
		}

		// Sort the slice by position
		sort.Slice(s.Dashboard.Widgets, func(i, j int) bool {
			return s.Dashboard.Widgets[i].Position < s.Dashboard.Widgets[j].Position
		})

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Bulk-save the entire dashboard widget list (used by edit-mode save).
	// When `cols` is present and layout_mode is per-breakpoint, saves to Layouts[cols].
	r.PUT("/api/dashboard/widgets", func(c *gin.Context) {
		var req struct {
			Cols    string `json:"cols"`
			Widgets []struct {
				ID       string `json:"id"`
				SizeName string `json:"size_name"`
			} `json:"widgets"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		list := make([]settings.DashboardWidget, 0, len(req.Widgets))
		seen := make(map[string]bool)
		for i, w := range req.Widgets {
			if _, ok := pm.Registry.Get(w.ID); !ok {
				continue
			}
			// Allow multiple instances of the same widget type (e.g. spacer:0, spacer:1)
			if seen[w.ID] {
				continue
			}
			seen[w.ID] = true
			list = append(list, settings.DashboardWidget{
				ID:       w.ID,
				SizeName: w.SizeName,
				Position: i,
			})
		}

		// Per-breakpoint: save to specific layout slot
		if req.Cols != "" && s.Dashboard.LayoutMode == "per-breakpoint" {
			if s.Dashboard.Layouts == nil {
				s.Dashboard.Layouts = make(map[string][]settings.DashboardWidget)
			}
			s.Dashboard.Layouts[req.Cols] = list
		} else {
			s.Dashboard.Widgets = list
		}

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Toggle layout mode between "auto" and "per-breakpoint"
	r.PUT("/api/dashboard/layout-mode", func(c *gin.Context) {
		var req struct {
			Mode string `json:"mode"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		if req.Mode != "auto" && req.Mode != "per-breakpoint" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be 'auto' or 'per-breakpoint'"})
			return
		}

		s.Dashboard.LayoutMode = req.Mode

		// When switching to per-breakpoint, bootstrap Layouts["5"] from current Widgets
		if req.Mode == "per-breakpoint" && len(s.Dashboard.Widgets) > 0 {
			if s.Dashboard.Layouts == nil {
				s.Dashboard.Layouts = make(map[string][]settings.DashboardWidget)
			}
			if _, ok := s.Dashboard.Layouts["5"]; !ok {
				cp := make([]settings.DashboardWidget, len(s.Dashboard.Widgets))
				copy(cp, s.Dashboard.Widgets)
				s.Dashboard.Layouts["5"] = cp
			}
		}

		if req.Mode == "auto" {
			s.Dashboard.Layouts = nil
		}

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "mode": req.Mode})
	})

	// Copy one breakpoint's layout to another
	r.POST("/api/dashboard/layouts/:cols/copy-from/:sourceCols", func(c *gin.Context) {
		cols := c.Param("cols")
		sourceCols := c.Param("sourceCols")

		if s.Dashboard.LayoutMode != "per-breakpoint" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "layout mode must be per-breakpoint"})
			return
		}

		if s.Dashboard.Layouts == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no layouts configured"})
			return
		}

		src, ok := s.Dashboard.Layouts[sourceCols]
		if !ok {
			// Fall back to default Widgets if source layout doesn't exist
			src = s.Dashboard.Widgets
		}

		cp := make([]settings.DashboardWidget, len(src))
		copy(cp, src)
		s.Dashboard.Layouts[cols] = cp

		if err := s.Save(); err != nil {
			respondError(c, logger, http.StatusInternalServerError, err, "failed to save settings")
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// start of default port (8080), will support opts later
	if err := r.Run(); err != nil {
		logger.Fatal().Err(err).Msg("failed to run server")
	}
}
