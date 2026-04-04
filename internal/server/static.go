package server

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pruvon/pruvon/static"

	"github.com/gofiber/fiber/v2"
)

// SetupStaticFileHandler sets up the static file handler for the application
func SetupStaticFileHandler(app *fiber.App) {
	app.Get("/static/*", handleStaticFile)
}

// handleStaticFile handles serving static files from embedded filesystem
func handleStaticFile(c *fiber.Ctx) error {
	// Get the requested file path from URL
	path := c.Params("*")

	// Adjust path for embedded files
	adjustedPath := path
	if strings.HasPrefix(path, "images/") {
		adjustedPath = path
	} else if strings.Contains(path, "/images/") {
		adjustedPath = strings.TrimPrefix(path, "/")
	}

	// Try to read the file from embedded filesystem
	content, err := static.StaticFiles.ReadFile(adjustedPath)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("File not found")
	}

	// Set the correct content type
	contentType := getContentType(path)
	c.Set("Content-Type", contentType)

	// Set caching headers based on file type
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case ext == ".js" || ext == ".css" || ext == ".svg" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".ico" || ext == ".webp" || ext == ".woff" || ext == ".woff2":
		// Cache static assets for 30 days (2592000 seconds)
		c.Set("Cache-Control", "public, max-age=2592000, stale-while-revalidate=86400")
		// Add ETag for validation
		etag := fmt.Sprintf(`"%x"`, sha1.Sum(content))
		c.Set("ETag", etag)
	default:
		// No cache for other file types
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}

	return c.Send(content)
}

// getContentType returns the content type based on file extension
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
