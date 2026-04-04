package web

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/system"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandleServices handles the services listing page
func HandleServices(c *fiber.Ctx) error {
	// Get all service types using the helper function
	svcTypes := dokku.GetServicePluginList()

	// Use the services template
	templateName := "services.html"

	tmpl, err := templates.GetTemplate(templateName)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)

	// Set the title
	sessionData["title"] = "Services"
	sessionData["serviceTypes"] = svcTypes
	sessionData["installedServiceTypes"] = svcTypes        // Just provide all types, will be filtered by API
	sessionData["allServices"] = make(map[string][]string) // Empty map, will be filled by API calls

	// Create an empty map for allowed services - will be populated by API calls
	sessionData["allowedServicesByType"] = make(map[string][]string)

	// Render template with data
	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleServiceDetail handles the service detail page
func HandleServiceDetail(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Use the service detail template
	templateName := "service_detail.html"

	tmpl, err := templates.GetTemplate(templateName)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	sessionData := GetSessionData(c)
	sessionData["Type"] = svcType
	sessionData["Name"] = svcName
	sessionData["LoadXTerm"] = true

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleServiceCreate handles the service creation page
func HandleServiceCreate(c *fiber.Ctx) error {
	// Set the appropriate back URL and title
	backURL := "/services"
	entityType := "Service"

	// Get service type from query parameter
	svcType := c.Query("type")

	// If no type is specified, show the service type selection page
	if svcType == "" {
		templateName := "create_service.html"

		tmpl, err := templates.GetTemplate(templateName)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
		}

		sessionData := GetSessionData(c)
		sessionData["title"] = fmt.Sprintf("Create %s", entityType)

		var out bytes.Buffer
		if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
		}

		return c.Type("html").SendString(out.String())
	}

	// Check if service type is valid
	validTypes := []string{"postgres", "mariadb", "mongo", "redis"}
	isValid := false
	// First check if the type is valid
	if system.Contains(validTypes, svcType) {
		// Then check if plugin is installed
		output, err := dokkuRunner.RunCommand("dokku", "plugin:list")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Plugin list could not be retrieved: %v", err),
			})
		}
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			pluginName := strings.Fields(line)
			if len(pluginName) > 0 {
				// Handle special case for mongo
				if svcType == "mongo" && pluginName[0] == "mongo" {
					isValid = true
					break
				}
				// Handle other service types
				if pluginName[0] == svcType {
					isValid = true
					break
				}
			}
		}
	}

	if !isValid {
		tmpl, err := templates.GetTemplate("error.html")
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
		}

		var message string
		if !system.Contains(validTypes, svcType) {
			message = fmt.Sprintf("The %s type '%s' is not supported.", strings.ToLower(entityType), svcType)
		} else {
			message = fmt.Sprintf("The plugin for %s is not installed. Please install it first.", svcType)
		}

		sessionData := GetSessionData(c)
		sessionData["Title"] = fmt.Sprintf("Invalid %s Type", entityType)
		sessionData["Message"] = message
		sessionData["BackURL"] = backURL
		sessionData["BackText"] = fmt.Sprintf("Back to %ss", entityType)

		var out bytes.Buffer
		if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
		}

		return c.Status(fiber.StatusNotFound).Type("html").SendString(out.String())
	}

	templateName := "create_service.html"

	tmpl, err := templates.GetTemplate(templateName)
	if err != nil {
		// Fall back to database template if service-specific one doesn't exist
		templateName = "create_database.html"
		tmpl, err = templates.GetTemplate(templateName)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
		}
	}

	sessionData := GetSessionData(c)
	sessionData["title"] = fmt.Sprintf("Create %s", entityType)
	sessionData["svcType"] = svcType

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}
