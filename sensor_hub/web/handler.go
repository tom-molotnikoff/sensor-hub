package web

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterSPAHandler(router *gin.Engine) {
	stripped, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem for embedded UI: " + err.Error())
	}
	RegisterSPAHandlerFS(router, stripped)
}

func RegisterSPAHandlerFS(router *gin.Engine, ui fs.FS) {
	fileServer := http.FileServer(http.FS(ui))

	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Never serve SPA fallback for API or docs routes
		if strings.HasPrefix(path, "/api/") || path == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if strings.HasPrefix(path, "/docs/") || path == "/docs" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Try to serve the exact file (JS, CSS, images, etc.)
		if f, err := ui.Open(strings.TrimPrefix(path, "/")); err == nil {
			f.Close()
			if strings.HasPrefix(path, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html for client-side routing
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
