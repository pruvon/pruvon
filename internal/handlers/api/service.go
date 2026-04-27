package api

import (
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/docker"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/handlers/web"
	internallog "github.com/pruvon/pruvon/internal/log"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	"github.com/pruvon/pruvon/internal/services"
	servicelogs "github.com/pruvon/pruvon/internal/services/logs"
	"github.com/pruvon/pruvon/internal/templates"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func SetupServiceRoutes(app *fiber.App) {
	// Routes for service APIs
	// Put specific routes BEFORE wildcard routes to prevent routing conflicts
	app.Get("/api/services/available", handleServiceAvailable)
	app.Get("/api/services/installed", handleInstalledServices)
	app.Get("/api/services/list", handleServiceList)

	// These wildcard routes should come after specific routes
	app.Get("/api/services/:type", handleServiceList)
	app.Get("/api/services/:type/:name/info", handleServiceInfo)
	app.Get("/api/services/:type/:name/basic-info", handleServiceBasicInfo)
	app.Get("/api/services/:type/:name/resources", handleServiceResourceInfo)
	app.Get("/api/services/:type/:name/links", handleServiceLinksInfo)
	app.Get("/api/services/:type/:name/download/:file", handleServiceDownload)
	app.Get("/api/services/:type/:name/export", handleServiceExport)
	app.Post("/api/services/:type/:name/export", handleServiceExport)

	app.Post("/api/services/create", handleServiceCreate)
	app.Post("/api/services/link", handleServiceLink)
	app.Delete("/api/services/link", handleServiceUnlink)
	app.Delete("/api/services/:type/:name", handleServiceDelete)
	app.Get("/api/services/:type/:name/config", handleServiceGetConfig)
	app.Post("/api/services/:type/:name/config", handleServiceSaveConfig)
	app.Post("/api/services/:type/:name/import", handleServiceImport)
	app.Post("/api/services/:type/:name/resource", handleServiceResourceSet)
}

func handleServiceList(c *fiber.Ctx) error {
	svcType := c.Params("type")

	// Make sure a service type is provided
	if svcType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Service type is required",
		})
	}

	// First, check if this service type is valid
	isValidServiceType := false
	allTypes := dokku.GetServicePluginList()
	for _, t := range allTypes {
		if t == svcType {
			isValidServiceType = true
			break
		}
	}

	if !isValidServiceType {
		// Return empty list for invalid service type
		return c.JSON(fiber.Map{
			"services": []models.Service{},
			"message":  fmt.Sprintf("Invalid service type: %s", svcType),
		})
	}

	// Check if plugin is installed for this service type - lighter check first
	hasPlugin, err := dokku.IsPluginInstalled(svcType)
	if err != nil {
		internallog.LogWarning(fmt.Sprintf("Error checking if plugin %s is installed: %v", svcType, err))
	}

	if !hasPlugin {
		// Return empty list instead of error if plugin isn't installed
		return c.JSON(fiber.Map{
			"services": []models.Service{},
			"message":  fmt.Sprintf("Plugin %s is not installed", svcType),
		})
	}

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if minimal mode is requested
	minimalMode := c.Query("minimal") == "true"

	var svcs []models.Service

	if minimalMode {
		// Try fast filesystem-based service name retrieval first
		serviceNames, err := dokku.GetServiceNamesByFilesystem(svcType)
		if err != nil {
			// Filesystem check failed, fall back to dokku command
			internallog.LogWarning(fmt.Sprintf("Filesystem check failed for %s services, using dokku command: %v", svcType, err))
			serviceNames, err = dokku.GetServiceNamesOnly(commandRunner, svcType)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
				})
			}
		} else {
			internallog.LogInfo(fmt.Sprintf("Retrieved %d service names for %s via filesystem check", len(serviceNames), svcType))
		}

		// Convert to basic service objects
		for _, name := range serviceNames {
			svcs = append(svcs, models.Service{
				Name: name,
			})
		}
	} else {
		// Full service info including status
		svcs, err = dokku.GetServices(commandRunner, svcType)
		if err != nil {
			errorMsg := fmt.Sprintf("Service list could not be retrieved: %v", err)
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:       time.Now(),
				RequestID:  uuid.New().String(),
				IP:         c.IP(),
				User:       "system",
				Action:     "service_list_error",
				Error:      errorMsg,
				Parameters: json.RawMessage(fmt.Sprintf(`{"type":"%s"}`, svcType)),
				StatusCode: 500,
			})
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": errorMsg,
				"debug": "Service list retrieval failed",
			})
		}
	}

	// Get all service names for permission filtering
	var svcNames []string
	for _, svc := range svcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Use the permission helper
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)

	// Filter service list based on permissions
	var filteredSvcs []models.Service
	for _, svc := range svcs {
		for _, allowedSvc := range allowedSvcs {
			if svc.Name == allowedSvc {
				filteredSvcs = append(filteredSvcs, svc)
				break
			}
		}
	}

	return c.JSON(fiber.Map{
		"services": filteredSvcs,
	})
}

func handleServiceInfo(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission to this service",
		})
	}

	info, err := dokku.GetServiceInfo(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service information could not be retrieved: %v", err),
		})
	}
	return c.JSON(info)
}

func handleServiceDownload(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")
	filename := c.Params("file")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission to this service backup file",
		})
	}

	filepath := fmt.Sprintf("/tmp/%s", filename)

	// Check if file exists before attempting to download
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": fmt.Sprintf("File not found: %s", filename),
		})
	}

	// Determine appropriate file extension and content type
	var contentType string
	if strings.HasSuffix(filename, ".sql") {
		contentType = "application/sql"
	} else if strings.HasSuffix(filename, ".rdb") {
		contentType = "application/octet-stream"
	} else if strings.HasSuffix(filename, ".archive") {
		contentType = "application/octet-stream"
	} else {
		contentType = "application/octet-stream"
	}

	if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".gz") {
		contentType = "application/gzip"
	}

	// Set proper content type
	c.Set("Content-Type", contentType)

	// Use the original filename from ExportService function for the download
	// filename includes the timestamp and proper extension already
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Remove the file after download
	defer os.Remove(filepath)

	// Send the file to the client
	return c.SendFile(filepath)
}

func handleServiceExport(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have permission to export this service",
		})
	}

	filename, err := dokku.ExportService(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service export failed: %v", err),
		})
	}

	// Create a download URL for the exported file
	downloadURL := fmt.Sprintf("/api/services/%s/%s/download/%s", svcType, svcName, filename)

	return c.JSON(fiber.Map{
		"filename":   filename,
		"export_url": downloadURL,
	})
}

func handleServiceAvailable(c *fiber.Ctx) error {
	// Use our new utility functions to get the list of all service plugins and available ones
	availableTypes, err := dokku.GetAvailableServicePluginList(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Plugin list could not be retrieved: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"services": availableTypes,
	})
}

func handleServiceCreate(c *fiber.Ctx) error {
	var req struct {
		Type         string `json:"type"`
		Name         string `json:"name"`
		ImageType    string `json:"imageType"`
		Image        string `json:"image"`
		ImageVersion string `json:"imageVersion"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Get session data for permission checking
	sessionData := web.GetSessionData(c)
	authType := sessionData["AuthType"]

	// Only admin users can create services
	if authType != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Admin privileges are required to create a service",
		})
	}

	_ = servicelogs.LogWithParams(c, "create_service", fiber.Map{
		"type":         req.Type,
		"name":         req.Name,
		"imageType":    req.ImageType,
		"image":        req.Image,
		"imageVersion": req.ImageVersion,
	})

	var output string
	var err error

	// Check if custom image and version are specified
	if req.Image != "" && req.ImageVersion != "" {
		// Using custom image for any service type
		args := []string{
			fmt.Sprintf("%s:create", req.Type),
			req.Name,
			"--image",
			req.Image,
			"--image-version",
			req.ImageVersion,
		}
		output, err = commandRunner.RunCommand("dokku", args...)
	} else {
		// Standard image
		output, err = commandRunner.RunCommand("dokku", fmt.Sprintf("%s:create", req.Type), req.Name)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": output,
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleServiceLink(c *fiber.Ctx) error {
	var req struct {
		Type    string `json:"type"`
		Service string `json:"service"`
		App     string `json:"app"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, req.Type)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, req.Type, svcNames)
	hasSvcAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == req.Service {
			hasSvcAccess = true
			break
		}
	}

	if !hasSvcAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have permission to link this service to apps",
		})
	}

	// Check if user has access to this app
	allApps, err := dokku.GetDokkuApps(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Application list could not be retrieved: %v", err),
		})
	}

	allowedApps := templates.GetUserAllowedApps(username, authType, allApps)
	hasAppAccess := false
	for _, allowedApp := range allowedApps {
		if allowedApp == req.App {
			hasAppAccess = true
			break
		}
	}

	if !hasAppAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission for this application",
		})
	}

	_ = servicelogs.LogWithParams(c, "link_service", fiber.Map{
		"type":    req.Type,
		"service": req.Service,
		"app":     req.App,
	})

	output, err := commandRunner.RunCommand("dokku", fmt.Sprintf("%s:link", req.Type), req.Service, req.App)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service link failed: %v", err),
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": output,
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleServiceUnlink(c *fiber.Ctx) error {
	var req struct {
		Type    string `json:"type"`
		Service string `json:"service"`
		App     string `json:"app"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, req.Type)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, req.Type, svcNames)
	hasSvcAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == req.Service {
			hasSvcAccess = true
			break
		}
	}

	if !hasSvcAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have permission to remove this service link",
		})
	}

	// Check if user has access to this app
	allApps, err := dokku.GetDokkuApps(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Application list could not be retrieved: %v", err),
		})
	}

	allowedApps := templates.GetUserAllowedApps(username, authType, allApps)
	hasAppAccess := false
	for _, allowedApp := range allowedApps {
		if allowedApp == req.App {
			hasAppAccess = true
			break
		}
	}

	if !hasAppAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission for this application",
		})
	}

	_ = servicelogs.LogWithParams(c, "unlink_service", fiber.Map{
		"type":    req.Type,
		"service": req.Service,
		"app":     req.App,
	})

	output, err := commandRunner.RunCommand("dokku", fmt.Sprintf("%s:unlink", req.Type), req.Service, req.App)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service unlink failed: %v", err),
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": output,
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleServiceDelete(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have permission to delete this service",
		})
	}

	// Check linked applications
	linkedApps, err := dokku.GetLinkedApps(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Linked apps check failed: %v", err),
		})
	}

	if len(linkedApps) > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Service has linked applications. Please unlink them first.",
		})
	}

	_ = servicelogs.LogWithParams(c, "delete_service", fiber.Map{
		"type": svcType,
		"name": svcName,
	})

	output, err := commandRunner.RunCommand("dokku", fmt.Sprintf("%s:destroy", svcType), svcName, "-f")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service could not be deleted: %v", err),
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": output,
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleServiceGetConfig(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission for this service configuration",
		})
	}

	// For PostgreSQL, we need to read custom.conf from the data directory
	if svcType == "postgres" {
		// Direct file access instead of running dokku command
		dataDir := filepath.Join("/var/lib/dokku/services/postgres", svcName, "data")

		// Check if the data directory exists
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Data directory not found: " + dataDir,
			})
		}

		// Path to custom.conf
		customConfPath := filepath.Join(dataDir, "custom.conf")

		// Read the custom.conf file if it exists
		configContent := ""
		if _, err := os.Stat(customConfPath); err == nil {
			// File exists, read it
			data, err := os.ReadFile(customConfPath)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("custom.conf file could not be read: %v", err),
				})
			}
			configContent = string(data)
		}

		// Return the config content
		return c.JSON(fiber.Map{
			"config": configContent,
		})
	}

	// For other services, use the existing implementation
	output, err := commandRunner.RunCommand("dokku", fmt.Sprintf("%s:info", svcType), svcName, "--config")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service configuration could not be retrieved: %v", err),
		})
	}

	lines := strings.Split(output, "\n")
	config := make(map[string]string)

	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				config[key] = value
			}
		}
	}

	return c.JSON(fiber.Map{
		"config": config,
	})
}

func handleServiceSaveConfig(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission checking
	sessionData := web.GetSessionData(c)
	authType := sessionData["AuthType"]

	// Only admin users can modify service configuration
	if authType != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Admin privileges are required to modify service configuration",
		})
	}

	// Special handling for PostgreSQL custom.conf
	if svcType == "postgres" {
		// Direct file access instead of running dokku command
		dataDir := filepath.Join("/var/lib/dokku/services/postgres", svcName, "data")

		// Check if the data directory exists
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Data directory not found: " + dataDir,
			})
		}

		// Path to custom.conf
		customConfPath := filepath.Join(dataDir, "custom.conf")

		// Parse request body as string
		var requestBody struct {
			Config string `json:"config"`
		}
		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request data",
			})
		}

		// Write custom.conf file
		err := os.WriteFile(customConfPath, []byte(requestBody.Config), 0644)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("custom.conf file could not be created: %v", err),
			})
		}

		// Restart the PostgreSQL service to apply configuration changes
		output, err := commandRunner.RunCommand("dokku", "postgres:restart", svcName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Configuration changes applied but service restart failed: %v", err),
			})
		}

		// Log restart action
		internallog.LogInfo(fmt.Sprintf("PostgreSQL service %s restarted after config change: %s", svcName, output))

		return c.SendStatus(fiber.StatusOK)
	}

	// For other services, use the existing implementation
	var config map[string]string
	if err := c.BodyParser(&config); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	for key, value := range config {
		_, err := commandRunner.RunCommand("dokku", fmt.Sprintf("%s:set", svcType), svcName, key, value)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Configuration could not be saved: %v", err),
			})
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

// handleServiceImport handles the initial request for service import
func handleServiceImport(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// --- Validate Service Type ---
	supportedTypes := []string{"postgres", "mariadb", "mongo", "redis", "rabbitmq", "memcached"}
	validType := false
	for _, t := range supportedTypes {
		if svcType == t {
			validType = true
			break
		}
	}
	if !validType {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Unsupported service type: %s. Supported types: %v", svcType, supportedTypes),
		})
	}

	// --- Permission Check (Admin Only) ---
	sessionData := web.GetSessionData(c)
	username, okUser := sessionData["username"].(string) // Type assertion
	authType, okAuth := sessionData["AuthType"].(string) // Type assertion

	if !okUser || !okAuth { // Check if assertions were successful
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read user session data",
		})
	}

	if authType != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Service import requires administrator privileges",
		})
	}

	// --- File Handling ---
	file, err := c.FormFile("backupFile") // Assuming the form field name is 'backupFile'
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get backup file: %v", err),
		})
	}

	// Check file size - 5GB limit
	const maxSize = 5 * 1024 * 1024 * 1024 // 5GB in bytes
	if file.Size > maxSize {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"error": fmt.Sprintf("File size (%.2f GB) exceeds the 5GB limit. Please use the command line for larger files.", float64(file.Size)/(1024*1024*1024)),
		})
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("/tmp", fmt.Sprintf("pruvon_import_%s_%s_*.sql", svcType, svcName))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to create temporary file: %v", err),
		})
	}
	tempFilePath := tempFile.Name()
	tempFile.Close() // Close immediately, we just need the path; SaveFile will handle writing

	// Save the uploaded file to the temporary path
	if err := c.SaveFile(file, tempFilePath); err != nil {
		os.Remove(tempFilePath) // Clean up temp file on error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to save uploaded file: %v", err),
		})
	}

	// --- Validate Plugin Installed ---
	pluginOutput, err := commandRunner.RunCommand("dokku", "plugin:list")
	if err != nil {
		os.Remove(tempFilePath) // Clean up temp file on error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to check installed plugins: %v", err),
		})
	}
	if !strings.Contains(pluginOutput, svcType) {
		os.Remove(tempFilePath) // Clean up temp file on error
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("%s plugin is not installed. Please install dokku-%s plugin first.", svcType, svcType),
		})
	}

	// --- Validate Service Exists ---
	output, err := commandRunner.RunCommand("dokku", fmt.Sprintf("%s:list", svcType))
	if err != nil {
		os.Remove(tempFilePath) // Clean up temp file on error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to list %s services: %v", svcType, err),
		})
	}
	if !strings.Contains(output, svcName) {
		os.Remove(tempFilePath) // Clean up temp file on error
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Service '%s' does not exist. Available %s services: %s", svcName, svcType, strings.TrimSpace(output)),
		})
	}

	// --- Task Management ---
	taskId := uuid.New().String()
	taskDetails := services.ImportTaskDetails{
		SvcType:          svcType,
		SvcName:          svcName,
		TempFilePath:     tempFilePath,
		OriginalFilename: file.Filename,
		User:             username,
		AuthType:         authType,
		StartTime:        time.Now(),
	}

	services.ImportTasksMutex.Lock()
	services.ImportTasks[taskId] = taskDetails
	services.ImportTasksMutex.Unlock()

	// Log the initiation of the import
	_ = servicelogs.LogWithParams(c, "service_import_start", fiber.Map{
		"type":     svcType,
		"name":     svcName,
		"taskId":   taskId,
		"filename": file.Filename,
	})

	// Return the task ID to the client
	return c.JSON(fiber.Map{
		"taskId": taskId,
	})
}

// handleServiceResourceSet updates resource limits for a service container
func handleServiceResourceSet(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")
	var req struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
	}

	if err := c.BodyParser(&req); err != nil {
		_ = middleware.SetFlashMessage(c, "Invalid request data", "error")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "Invalid request data",
			"message":    "Invalid request data",
			"flash_type": "error",
		})
	}

	// Check if at least one resource limit is provided
	if req.CPU == "" && req.Memory == "" {
		_ = middleware.SetFlashMessage(c, "At least one resource limit must be specified (CPU or Memory)", "error")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "At least one resource limit must be provided (CPU or Memory)",
			"message":    "At least one resource limit must be provided (CPU or Memory)",
			"flash_type": "error",
		})
	}

	// Check authorization
	sessionData := web.GetSessionData(c)
	if sessionData == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":      "Unauthorized access",
			"message":    "Unauthorized access",
			"flash_type": "error",
		})
	}

	username := sessionData["Username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("Service list could not be retrieved: %v", err),
			"message":    fmt.Sprintf("Service list could not be retrieved: %v", err),
			"flash_type": "error",
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":      "You do not have permission to change resource limits for this service",
			"message":    "You do not have permission to change resource limits for this service",
			"flash_type": "error",
		})
	}

	// Get service info to get container ID
	serviceInfo, err := dokku.GetServiceInfo(commandRunner, svcType, svcName)
	if err != nil {
		errorMsg := fmt.Sprintf("Service %s information could not be retrieved", svcName)
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("Failed to get service info for %s: %v", svcName, err),
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	// Check if we have a container ID
	if serviceInfo.ContainerID == "" {
		errorMsg := fmt.Sprintf("Container ID not found for service %s", svcName)
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("Container ID not found for service %s", svcName),
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	// Update resource limits using the Docker update command
	err = docker.UpdateContainerResourceLimits(commandRunner, serviceInfo.ContainerID, req.CPU, req.Memory)
	if err != nil {
		errorMsg := fmt.Sprintf("Error updating resource limits for container %s: %v", serviceInfo.ContainerID, err)
		_ = middleware.SetFlashMessage(c, errorMsg, "error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("Failed to update resource limits for container %s: %v", serviceInfo.ContainerID, err),
			"message":    errorMsg,
			"flash_type": "error",
		})
	}

	// Set success message
	successMsg := fmt.Sprintf("Resource limits for service %s have been updated", svcName)
	_ = middleware.SetFlashMessage(c, successMsg, "success")

	return c.JSON(fiber.Map{
		"success":    true,
		"message":    successMsg,
		"flash_type": "success",
	})
}

// handleServiceBasicInfo returns only essential information about a service for fast loading
func handleServiceBasicInfo(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission for this service",
		})
	}

	// Get only basic service info (avoiding expensive operations)
	info, err := dokku.GetServiceBasicInfo(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service basic info could not be retrieved: %v", err),
		})
	}

	// Return only essential information
	return c.JSON(fiber.Map{
		"name":    info.Name,
		"status":  info.Status,
		"version": info.Version,
	})
}

// handleServiceResourceInfo returns only resource limit info for a service
func handleServiceResourceInfo(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission for this service",
		})
	}

	// Get only resource info
	resourceInfo, err := dokku.GetServiceResourceInfo(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service resource info could not be retrieved: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"resourceLimits": resourceInfo,
	})
}

// handleServiceLinksInfo returns only linked apps info for a service
func handleServiceLinksInfo(c *fiber.Ctx) error {
	svcType := c.Params("type")
	svcName := c.Params("name")

	// Get session data for permission filtering
	sessionData := web.GetSessionData(c)
	username := sessionData["username"]
	authType := sessionData["AuthType"]

	// Check if user has access to this service
	allSvcs, err := dokku.GetServices(commandRunner, svcType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Service list could not be retrieved: %v", err),
		})
	}

	// Get all service names
	var svcNames []string
	for _, svc := range allSvcs {
		svcNames = append(svcNames, svc.Name)
	}

	// Check if user has access to this service
	allowedSvcs := templates.GetUserAllowedServices(username, authType, svcType, svcNames)
	hasAccess := false
	for _, allowedSvc := range allowedSvcs {
		if allowedSvc == svcName {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access permission for this service",
		})
	}

	// Get only linked apps
	linkedApps, err := dokku.GetLinkedApps(commandRunner, svcType, svcName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Linked apps could not be retrieved: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"linkedApps": linkedApps,
	})
}

// handleInstalledServices returns a list of service plugins that are actually installed on the system
func handleInstalledServices(c *fiber.Ctx) error {
	// Try the fast filesystem-based check first
	internallog.LogInfo("Getting installed service plugins using filesystem check")
	installedTypes, err := dokku.GetInstalledServicePluginsByFilesystem()

	if err != nil {
		// Filesystem check failed, fall back to dokku command
		internallog.LogWarning(fmt.Sprintf("Filesystem check failed, falling back to dokku command: %v", err))
		return handleInstalledServicesFallback(c)
	}

	// If filesystem check succeeded
	if len(installedTypes) == 0 {
		internallog.LogInfo("No installed service plugins found via filesystem check")
	} else {
		internallog.LogInfo(fmt.Sprintf("Found installed service plugins via filesystem: %v", installedTypes))
	}

	return c.JSON(fiber.Map{
		"services": installedTypes,
	})
}

// handleInstalledServicesFallback is the original implementation using dokku commands
// Used as a fallback when filesystem checks fail
func handleInstalledServicesFallback(c *fiber.Ctx) error {
	// List of all possible service types
	allTypes := []string{
		"postgres", "mariadb", "mongo", "redis", "rabbitmq", "memcached",
		"clickhouse", "elasticsearch", "nats", "solr", "rethinkdb",
		"couchdb", "meilisearch", "pushpin", "omnisci",
	}

	// Log what we're doing
	internallog.LogInfo("Getting installed service plugins using dokku plugin:list (fallback)")

	// Get the list of all installed plugins using 'dokku plugin:list'
	output, err := commandRunner.RunCommand("dokku", "plugin:list")
	if err != nil {
		internallog.LogError(fmt.Sprintf("Failed to get installed plugins list: %v", err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":    fmt.Sprintf("Failed to get installed plugins: %v", err),
			"services": []string{},
		})
	}

	// Log the raw output for debugging
	internallog.LogInfo(fmt.Sprintf("Raw plugin list output: %s", output))

	// Parse the output to get plugin names
	var installedTypes []string
	lines := strings.Split(output, "\n")

	// Create a map for easier lookup
	serviceTypesMap := make(map[string]bool)
	for _, svcType := range allTypes {
		serviceTypesMap[svcType] = true
	}

	// Process each line of the plugin list output
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract plugin name - the output format is usually "plugin_name  version"
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		pluginName := parts[0]

		// Check if the plugin name matches one of our predefined service types directly
		if serviceTypesMap[pluginName] {
			internallog.LogInfo(fmt.Sprintf("Found installed service plugin: %s", pluginName))
			installedTypes = append(installedTypes, pluginName)
			continue
		}

		// Also check for the legacy format with "service-" prefix
		if strings.HasPrefix(pluginName, "service-") {
			// Extract the service type name (remove "service-" prefix)
			serviceType := strings.TrimPrefix(pluginName, "service-")

			// Only add if it's in our predefined allTypes list
			if serviceTypesMap[serviceType] {
				internallog.LogInfo(fmt.Sprintf("Found installed service plugin: %s", serviceType))
				installedTypes = append(installedTypes, serviceType)
			}
		}
	}

	// If no installed service plugins were found, log it
	if len(installedTypes) == 0 {
		internallog.LogWarning("No installed service plugins found in dokku plugin:list output")
	} else {
		internallog.LogInfo(fmt.Sprintf("Found installed service plugins: %v", installedTypes))
	}

	return c.JSON(fiber.Map{
		"services": installedTypes,
	})
}
