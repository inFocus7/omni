package main

import (
	"context"
	"log"
	"net/http"
	"net/url"

	"github.com/infocus7/dashie/pkg/plugins"
	"github.com/infocus7/dashie/pkg/settings"
	"github.com/infocus7/dashie/ui"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()

	s, err := settings.Load()
	if err != nil {
		log.Printf("warning: could not load settings: %v", err)
		s = &settings.Settings{}
	}

	pm, err := plugins.NewPluginManager(ctx, s)
	if err != nil {
		panic(err)
	}

	r := gin.Default()
	r.Static("/static", "./ui/static")

	r.GET("/", func(c *gin.Context) {
		filter := c.DefaultQuery("filter", "7d")

		data, err := pm.FetchDashboardData(filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		pages, err := ui.Pages()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := pages.ExecuteTemplate(c.Writer, "dashboard.tmpl", data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	r.GET("/github", func(c *gin.Context) {
		filter := c.DefaultQuery("filter", "7d")

		data, err := pm.FetchGitHubDetail(filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		pages, err := ui.Pages()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := pages.ExecuteTemplate(c.Writer, "github_page.tmpl", data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	r.GET("/settings", func(c *gin.Context) {
		section := c.DefaultQuery("section", "general")

		pages, err := ui.Pages()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		data := gin.H{
			"Section":  section,
			"Settings": s,
		}

		if err := pages.ExecuteTemplate(c.Writer, "settings_page.tmpl", data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	})

	r.POST("/settings/github/watch", func(c *gin.Context) {
		raw := c.PostForm("entry")
		entry := settings.NormalizeEntry(raw)
		if entry == "" {
			c.Redirect(http.StatusSeeOther, "/settings?section=github")
			return
		}

		// Deduplicate
		for _, w := range s.GitHub.Watched {
			if w == entry {
				c.Redirect(http.StatusSeeOther, "/settings?section=github")
				return
			}
		}

		s.GitHub.Watched = append(s.GitHub.Watched, entry)
		if err := s.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Redirect(http.StatusSeeOther, "/settings?section=github")
	})

	r.DELETE("/settings/github/watch/:entry", func(c *gin.Context) {
		entry, err := url.PathUnescape(c.Param("entry"))
		if err != nil {
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Status(http.StatusNoContent)
	})

	// start of default port (8080), will support opts later
	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}
