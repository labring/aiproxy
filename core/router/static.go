package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SetStaticFileRouter configures routes to serve frontend static files
func SetStaticFileRouter(router *gin.Engine) {
	// Serve static assets
	router.Static("/assets", "./web/dist/assets")

	// Serve localization files
	router.Static("/locales", "./web/dist/locales")

	// Serve other static files
	router.StaticFile("/logo.svg", "./web/dist/logo.svg")

	// Handle non-API routes, returning the frontend entry point
	router.NoRoute(func(c *gin.Context) {
		// Return 404 for API requests
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
			return
		}

		// Return the frontend entry file for all other routes
		c.File("./web/dist/index.html")
	})
}
