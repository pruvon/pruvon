package api

import (
	"encoding/json"
	"github.com/pruvon/pruvon/static"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Template logo mappings - where to find the logos for each template
var templateLogos = map[string]string{
	"metabase":    "/static/images/logo/metabase.svg",
	"rails":       "/static/images/logo/rails.svg",
	"wordpress":   "/static/images/logo/wordpress.svg",
	"minio":       "/static/images/logo/minio.svg",
	"moodle":      "/static/images/logo/moodle.svg",
	"uptime-kuma": "/static/images/logo/uptime-kuma.svg",
}

// SetupTemplatesRoutes sets up routes for working with application templates
func SetupTemplatesRoutes(app *fiber.App) {
	templateGroup := app.Group("/api/templates")

	// Route to list available templates
	templateGroup.Get("/list", handleListTemplates)

	// Route to get a specific template
	templateGroup.Get("/load/:name", handleLoadTemplate)
}

// handleListTemplates returns a list of available app templates
func handleListTemplates(c *fiber.Ctx) error {
	log.Printf("Looking for embedded templates")

	var templates []map[string]string
	var foundTemplates = make(map[string]bool) // Track unique templates

	// Use embedded filesystem to find templates
	err := fs.WalkDir(static.EmbeddedTemplates, "app_templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error walking embedded templates: %v", err)
			return nil // Continue walking despite errors
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process JSON files
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		log.Printf("Found template file: %s", path)
		templateName := strings.TrimSuffix(filepath.Base(path), ".json")

		// Skip if we've already found this template
		if foundTemplates[templateName] {
			return nil
		}
		foundTemplates[templateName] = true

		// Read the template file
		content, err := static.EmbeddedTemplates.ReadFile(path)
		if err != nil {
			log.Printf("Error reading template file %s: %v", path, err)
			return nil
		}

		// Verify JSON is valid
		var templateData map[string]interface{}
		if err := json.Unmarshal(content, &templateData); err != nil {
			log.Printf("Invalid JSON in template file %s: %v", path, err)
			return nil
		}

		template := map[string]string{
			"name": templateName,
			"logo": templateLogos[templateName],
		}
		templates = append(templates, template)
		log.Printf("Found valid template: %s", templateName)

		return nil
	})

	if err != nil {
		log.Printf("Error walking embedded templates directory: %v", err)
	}

	log.Printf("Total templates found: %d", len(templates))
	if len(templates) == 0 {
		log.Printf("No templates found in embedded filesystem")
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"templates": templates,
	})
}

// handleLoadTemplate loads a specific template
func handleLoadTemplate(c *fiber.Ctx) error {
	// Get template name from URL
	templateName := c.Params("name")
	if templateName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Template name is required",
		})
	}

	// Ensure the template name is safe (no directory traversal)
	templateName = filepath.Base(templateName)
	log.Printf("Loading template: %s", templateName)

	// Try to find the template in the embedded filesystem
	filePath := filepath.Join("app_templates", templateName+".json")
	fileContent, err := static.EmbeddedTemplates.ReadFile(filePath)
	if err != nil {
		log.Printf("Template not found in embedded filesystem: %s, error: %v", filePath, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Template not found",
		})
	}

	// Parse the JSON to ensure it's valid
	var templateData map[string]interface{}
	if err := json.Unmarshal(fileContent, &templateData); err != nil {
		log.Printf("Error parsing template JSON: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid template JSON: " + err.Error(),
		})
	}

	log.Printf("Successfully loaded template: %s", templateName)

	return c.JSON(fiber.Map{
		"success":  true,
		"template": templateData,
	})
}
