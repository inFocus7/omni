package main

import (
	"context"
	"log"

	"net/http"

	"github.com/infocus7/dashie/pkg/plugins"
	"github.com/infocus7/dashie/ui"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	pm, err := plugins.NewPluginManager(ctx)
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

	// start of default port (8080), will support opts later
	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}
