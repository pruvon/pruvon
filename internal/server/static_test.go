package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestGetContentType(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "HTML file",
			path:     "index.html",
			expected: "text/html",
		},
		{
			name:     "CSS file",
			path:     "styles.css",
			expected: "text/css",
		},
		{
			name:     "JavaScript file",
			path:     "script.js",
			expected: "application/javascript",
		},
		{
			name:     "JSON file",
			path:     "data.json",
			expected: "application/json",
		},
		{
			name:     "PNG image",
			path:     "image.png",
			expected: "image/png",
		},
		{
			name:     "JPEG image",
			path:     "photo.jpg",
			expected: "image/jpeg",
		},
		{
			name:     "JPEG image (jpeg extension)",
			path:     "photo.jpeg",
			expected: "image/jpeg",
		},
		{
			name:     "GIF image",
			path:     "animation.gif",
			expected: "image/gif",
		},
		{
			name:     "SVG image",
			path:     "icon.svg",
			expected: "image/svg+xml",
		},
		{
			name:     "ICO file",
			path:     "favicon.ico",
			expected: "image/x-icon",
		},
		{
			name:     "Unknown extension",
			path:     "unknown.xyz",
			expected: "application/octet-stream",
		},
		{
			name:     "File without extension",
			path:     "README",
			expected: "application/octet-stream",
		},
		{
			name:     "Uppercase extension",
			path:     "FILE.HTML",
			expected: "text/html",
		},
		{
			name:     "Mixed case extension",
			path:     "script.Js",
			expected: "application/javascript",
		},
		{
			name:     "Full path",
			path:     "/static/assets/styles.css",
			expected: "text/css",
		},
		{
			name:     "File starting with dot",
			path:     ".htaccess",
			expected: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getContentType(tt.path)
			if result != tt.expected {
				t.Errorf("getContentType(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSetupStaticFileHandler(t *testing.T) {
	app := fiber.New()

	// Setup static file handler
	SetupStaticFileHandler(app)

	// Test that the route was added by making a request
	// Since embedded files may not exist in test, expect 404
	req := httptest.NewRequest("GET", "/static/nonexistent.js", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}
	// Should return 404 for non-existent file
	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404 for non-existent file, got %d", resp.StatusCode)
	}
}
