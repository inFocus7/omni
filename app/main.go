package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/infocus7/dash/pkg/plugins"
	"github.com/infocus7/dash/pkg/settings"
	"github.com/infocus7/dash/ui"

	"github.com/gin-gonic/gin"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
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
		logger.Error().Err(err).Msg("failed to initialize plugin manager")
		panic(err)
	}

	pages, err := ui.Pages()
	if err != nil {
		logger.Error().Err(err).Msg("failed to load UI pages")
		panic(err)
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

	r.GET("/", func(c *gin.Context) {
		filter := c.DefaultQuery("filter", "7d")

		data, err := pm.FetchDashboardData(filter)
		if err != nil {
			logger.Error().Err(err).Fields(map[string]interface{}{
				"filter": filter,
			}).Msg("failed to fetch dashboard data")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := pages.ExecuteTemplate(c.Writer, "dashboard.tmpl", data); err != nil {
			logger.Error().Err(err).Fields(map[string]interface{}{
				"filter": filter,
			}).Msg("failed to render dashboard template")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	r.GET("/plugins", func(c *gin.Context) {
		if err := pages.ExecuteTemplate(c.Writer, "plugins_page.tmpl", nil); err != nil {
			logger.Error().Err(err).Msg("failed to render plugins page template")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		if err := pages.ExecuteTemplate(c.Writer, "github_page.tmpl", data); err != nil {
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

		if err := pages.ExecuteTemplate(c.Writer, "settings_page.tmpl", data); err != nil {
			logger.Error().Err(err).Msg("failed to render settings page template")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		for _, w := range s.GitHub.Watched {
			if w == entry {
				c.Redirect(http.StatusSeeOther, "/settings?section=plugins")
				return
			}
		}

		s.GitHub.Watched = append(s.GitHub.Watched, entry)
		if err := s.Save(); err != nil {
			logger.Error().Err(err).Msg("failed to save settings")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		watched := s.GitHub.Watched[:0]
		for _, w := range s.GitHub.Watched {
			if w != entry {
				watched = append(watched, w)
			}
		}
		s.GitHub.Watched = watched

		if err := s.Save(); err != nil {
			logger.Error().Err(err).Msg("failed to save settings")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	})

	// start of default port (8080), will support opts later
	if err := r.Run(); err != nil {
		logger.Error().Err(err).Msg("failed to run server")
		os.Exit(1)
	}
}
