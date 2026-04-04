package web

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/docker"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandleDashboard handles the dashboard page
func HandleDashboard(c *fiber.Ctx) error {
	// Get Dokku version
	dokkuVersion, err := dokku.GetDokkuVersion(dokkuRunner)
	if err != nil {
		dokkuVersion = "Unknown"
	}

	// Get Docker stats
	dockerStats, err := docker.GetDockerStats(dokkuRunner)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Docker statistics could not be retrieved: "+err.Error())
	}

	// Get app count
	apps, err := dokku.GetDokkuApps(dokkuRunner)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Application list could not be retrieved: "+err.Error())
	}

	// Get plugin count
	plugins, err := dokku.GetPlugins(dokkuRunner)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Plugin list could not be retrieved: "+err.Error())
	}

	// Render template
	tmpl, err := templates.GetTemplate("dashboard.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)

	// Add dashboard specific data
	sessionData["Title"] = "Dashboard"
	sessionData["DokkuVersion"] = dokkuVersion
	sessionData["DockerStats"] = dockerStats
	sessionData["AppCount"] = len(apps)
	sessionData["PluginCount"] = len(plugins)
	sessionData["LoadApexCharts"] = true

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleApps handles the apps listing page
func HandleApps(c *fiber.Ctx) error {
	// Get apps list
	apps, err := dokku.GetDokkuApps(dokkuRunner)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "apps list error: "+err.Error())
	}

	// Render template
	tmpl, err := templates.GetTemplate("apps.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)

	// Add apps specific data
	sessionData["Title"] = "Applications"

	// Get username and auth type
	username := sessionData["Username"]
	if username == nil {
		// Try alternate key (for backward compatibility)
		username = sessionData["User"]
	}
	authType := sessionData["AuthType"]

	// The template will handle filtering using the getUserAllowedApps function
	sessionData["AppNames"] = apps
	sessionData["AllAppNames"] = apps // Store all apps for reference

	// Add information about user permissions and apps
	if cfg := c.Locals("config").(*config.Config); cfg != nil && authType == "github" {
		// Find user in config
		if usernameStr, ok := username.(string); ok {
			for _, user := range cfg.GitHub.Users {
				if user.Username == usernameStr {
					sessionData["UserHasApps"] = len(user.Apps) > 0
					sessionData["UserApps"] = user.Apps
					sessionData["UserRoutes"] = user.Routes
					break
				}
			}
		}
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleAppCreate handles the app creation page
func HandleAppCreate(c *fiber.Ctx) error {
	// Render template
	tmpl, err := templates.GetTemplate("create_app.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)

	// Add app create specific data
	sessionData["Title"] = "Create Application"

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleAppCreateSuccess handles the success page after app creation
func HandleAppCreateSuccess(c *fiber.Ctx) error {
	appName := c.Query("name")
	if appName != "" {
		_ = middleware.SetFlashMessage(c, fmt.Sprintf("Application '%s' created successfully", appName), "success")
	}
	return c.Redirect("/apps")
}

// HandleAppDetail handles the app detail page
func HandleAppDetail(c *fiber.Ctx) error {
	appName := c.Params("name")

	// Get app.json for description
	appJson, err := dokku.GetAppJson(dokkuRunner, appName)
	var description, version, repositoryURL string
	if err == nil && appJson != "" {
		description, version, repositoryURL = parseAppMetadata(appJson)
	}

	// Render template
	tmpl, err := templates.GetTemplate("app_detail.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)

	// Add app detail specific data
	sessionData["Title"] = fmt.Sprintf("App: %s", appName)
	sessionData["AppName"] = appName
	sessionData["Name"] = appName
	sessionData["Description"] = description
	sessionData["Version"] = version
	sessionData["RepositoryURL"] = repositoryURL
	sessionData["CreatedAt"] = dokku.GetAppCreatedAt(dokkuRunner, appName)
	sessionData["LastDeployAt"] = dokku.GetLastDeployTime(dokkuRunner, appName)
	sessionData["LoadXTerm"] = true

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

func parseAppMetadata(appJSON string) (description, version, repositoryURL string) {
	var manifest struct {
		Description string      `json:"description"`
		Version     string      `json:"version"`
		Repository  interface{} `json:"repository"`
	}

	if err := json.Unmarshal([]byte(appJSON), &manifest); err != nil {
		return "", "", ""
	}

	repositoryURL = extractRepositoryURL(manifest.Repository)
	return manifest.Description, manifest.Version, repositoryURL
}

func extractRepositoryURL(repository interface{}) string {
	switch value := repository.(type) {
	case string:
		return value
	case map[string]interface{}:
		url, _ := value["url"].(string)
		return url
	default:
		return ""
	}
}
