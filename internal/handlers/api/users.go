package api

import (
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	internallog "github.com/pruvon/pruvon/internal/log"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	servicelogs "github.com/pruvon/pruvon/internal/services/logs"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func SetupUsersRoutes(app *fiber.App) {
	app.Get("/api/settings/users", handleGetUsers)
	app.Get("/api/settings/user-options", handleGetUserOptions)
	app.Post("/api/settings/users/admin", handleUpdateAdminCredentials)
	app.Post("/api/settings/users/github", handleAddGithubUser)
	app.Delete("/api/settings/users/github/:username", handleDeleteGithubUser)
	app.Put("/api/settings/users/github/:username/routes", handleUpdateGithubUserRoutes)
}

func handleGetUsers(c *fiber.Ctx) error {
	cfg := c.Locals("config").(*config.Config)

	// Get the user from the session instead of Locals
	sess, _ := middleware.GetStore().Get(c)
	username := sess.Get("username")
	if username == nil {
		username = sess.Get("user")
	}

	// Default to "unknown" only if we couldn't get the username from session
	user := "unknown"
	if username != nil {
		user = username.(string)
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:      time.Now(),
		RequestID: uuid.New().String(),
		IP:        c.IP(),
		User:      user,
		Action:    "user_management_accessed",
		Method:    c.Method(),
		Route:     c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{
			"admin": "%s"
		}`, cfg.Admin.Username)),
		StatusCode: 200,
	})

	return c.JSON(fiber.Map{
		"admin": fiber.Map{
			"username": cfg.Admin.Username,
		},
		"github": fiber.Map{
			"users": cfg.GitHub.Users,
		},
	})
}

func handleUpdateAdminCredentials(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request",
		})
	}

	if req.Username == "" || req.Password == "" {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			User:      req.Username,
			Action:    "admin_credentials_update_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"password": "REDACTED",
				"reason": "missing_fields"
			}`, req.Username)),
			StatusCode: 400,
		})

		return c.Status(400).JSON(fiber.Map{
			"errors": fiber.Map{
				"username": "Username is required",
				"password": "Password is required",
			},
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			User:      req.Username,
			Action:    "admin_credentials_update_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"password": "REDACTED",
				"reason": "password_hash_failed"
			}`, req.Username)),
			StatusCode: 500,
		})
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to hash password",
		})
	}

	cfg := c.Locals("config").(*config.Config)
	oldUsername := cfg.Admin.Username

	cfg.Admin.Username = req.Username
	cfg.Admin.Password = string(hashedPassword)

	if err := config.SaveConfig(cfg); err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			User:      req.Username,
			Action:    "admin_credentials_update_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"old_username": "%s",
				"new_username": "%s",
				"password": "REDACTED",
				"reason": "save_config_failed"
			}`, oldUsername, req.Username)),
			StatusCode: 500,
		})
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save config",
		})
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:      time.Now(),
		RequestID: uuid.New().String(),
		IP:        c.IP(),
		User:      req.Username,
		Action:    "admin_credentials_updated",
		Method:    c.Method(),
		Route:     c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{
			"old_username": "%s",
			"new_username": "%s",
			"password": "REDACTED"
		}`, oldUsername, req.Username)),
		StatusCode: 200,
	})

	return c.SendStatus(200)
}

func handleAddGithubUser(c *fiber.Ctx) error {
	var req struct {
		Username string              `json:"username"`
		Routes   []string            `json:"routes"`
		Apps     []string            `json:"apps"`
		Services map[string][]string `json:"services"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request",
		})
	}

	if req.Username == "" {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_add_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(`{
				"reason": "missing_username"
			}`),
			StatusCode: 400,
		})

		return c.Status(400).JSON(fiber.Map{
			"errors": fiber.Map{
				"username": "Username is required",
			},
		})
	}

	if len(req.Routes) == 0 && len(req.Apps) == 0 {
		req.Routes = []string{"/*"}
	}

	cfg := c.Locals("config").(*config.Config)

	for _, user := range cfg.GitHub.Users {
		if user.Username == req.Username {
			_ = servicelogs.LogActivity(models.ActivityLog{
				Time:      time.Now(),
				RequestID: uuid.New().String(),
				IP:        c.IP(),
				Action:    "github_user_add_failed",
				Method:    c.Method(),
				Route:     c.Path(),
				Parameters: json.RawMessage(fmt.Sprintf(`{
					"username": "%s",
					"reason": "user_already_exists"
				}`, req.Username)),
				StatusCode: 400,
			})

			return c.Status(400).JSON(fiber.Map{
				"errors": fiber.Map{
					"username": "User already exists",
				},
			})
		}
	}

	// Generate app routes and add them
	var appRoutes []string
	for _, app := range req.Apps {
		// Special case for wildcard
		if app == "*" {
			appRoutes = append(appRoutes, "/apps/*")
			appRoutes = append(appRoutes, "/api/apps/*")
			appRoutes = append(appRoutes, "/ws/apps/*")
		}
		// Don't generate route paths for individual apps anymore
	}

	// Add the app-generated routes to the explicit routes
	routes := req.Routes
	if len(appRoutes) > 0 {
		// This is a set-like map to avoid duplicates
		routeSet := make(map[string]bool)

		// Add existing routes
		for _, route := range routes {
			routeSet[route] = true
		}

		// Add wildcard app routes only
		for _, route := range appRoutes {
			routeSet[route] = true
		}

		// Convert back to slice
		routes = make([]string, 0, len(routeSet))
		for route := range routeSet {
			routes = append(routes, route)
		}
	}

	// Generate service routes and add them
	var svcRoutes []string
	for svcType, services := range req.Services {
		for _, svc := range services {
			// Special case for wildcard
			if svc == "*" {
				// Generate routes for this service type
				svcRoutes = append(svcRoutes, fmt.Sprintf("/services/%s/*", svcType))
				svcRoutes = append(svcRoutes, fmt.Sprintf("/api/services/%s/*", svcType))
			}
			// Don't generate route paths for individual services anymore
		}
	}

	// Add the service-generated routes to the explicit routes
	if len(svcRoutes) > 0 {
		// This is a set-like map to avoid duplicates
		routeSet := make(map[string]bool)

		// Add existing routes
		for _, route := range routes {
			routeSet[route] = true
		}

		// Add wildcard service routes only
		for _, route := range svcRoutes {
			routeSet[route] = true
		}

		// Convert back to slice
		routes = make([]string, 0, len(routeSet))
		for route := range routeSet {
			routes = append(routes, route)
		}
	}

	// Create the new user
	newUser := config.GitHubUser{
		Username: req.Username,
		Routes:   routes,
		Apps:     req.Apps,
		Services: req.Services,
	}

	cfg.GitHub.Users = append(cfg.GitHub.Users, newUser)

	if err := config.SaveConfig(cfg); err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_add_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"reason": "save_config_failed"
			}`, req.Username)),
			StatusCode: 500,
		})

		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save config",
		})
	}

	// Get the user from the session instead of Locals
	sess, _ := middleware.GetStore().Get(c)
	sessionUsername := sess.Get("username")
	if sessionUsername == nil {
		sessionUsername = sess.Get("user")
	}

	// Default to "unknown" only if we couldn't get the username from session
	user := "unknown"
	if sessionUsername != nil {
		user = sessionUsername.(string)
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:      time.Now(),
		RequestID: uuid.New().String(),
		IP:        c.IP(),
		User:      user,
		Action:    "github_user_added",
		Method:    c.Method(),
		Route:     c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{
			"username": "%s"
		}`, req.Username)),
		StatusCode: 200,
	})

	return c.SendStatus(200)
}

func handleDeleteGithubUser(c *fiber.Ctx) error {
	cfg := c.Locals("config").(*config.Config)
	username := c.Params("username")

	newUsers := make([]config.GitHubUser, 0)
	found := false
	for _, user := range cfg.GitHub.Users {
		if user.Username != username {
			newUsers = append(newUsers, user)
		} else {
			found = true
		}
	}

	if !found {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_remove_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"reason": "user_not_found"
			}`, username)),
			StatusCode: 404,
		})

		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	cfg.GitHub.Users = newUsers

	if err := config.SaveConfig(cfg); err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_remove_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"reason": "save_config_failed"
			}`, username)),
			StatusCode: 500,
		})

		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save config",
		})
	}

	// Get the user from the session instead of Locals
	sess, _ := middleware.GetStore().Get(c)
	sessionUsername := sess.Get("username")
	if sessionUsername == nil {
		sessionUsername = sess.Get("user")
	}

	// Default to "unknown" only if we couldn't get the username from session
	user := "unknown"
	if sessionUsername != nil {
		user = sessionUsername.(string)
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:      time.Now(),
		RequestID: uuid.New().String(),
		IP:        c.IP(),
		User:      user,
		Action:    "github_user_removed",
		Method:    c.Method(),
		Route:     c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{
			"username": "%s"
		}`, username)),
		StatusCode: 200,
	})

	return c.SendStatus(200)
}

func handleUpdateGithubUserRoutes(c *fiber.Ctx) error {
	username := c.Params("username")
	var req struct {
		Routes   []string            `json:"routes"`
		Apps     []string            `json:"apps"`
		Services map[string][]string `json:"services"`
	}

	if err := c.BodyParser(&req); err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_routes_update_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"reason": "invalid_request"
			}`, username)),
			StatusCode: 400,
		})
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request",
		})
	}

	cfg := c.Locals("config").(*config.Config)

	userFound := false
	oldRoutes := []string{}
	oldApps := []string{}
	oldServices := map[string][]string{}

	for i, user := range cfg.GitHub.Users {
		if user.Username == username {
			oldRoutes = user.Routes
			oldApps = user.Apps
			oldServices = user.Services

			// Update routes
			cfg.GitHub.Users[i].Routes = req.Routes

			// Update apps
			cfg.GitHub.Users[i].Apps = req.Apps

			// Generate app routes and add them
			var appRoutes []string
			for _, app := range req.Apps {
				// Special case for wildcard
				if app == "*" {
					appRoutes = append(appRoutes, "/apps/*")
					appRoutes = append(appRoutes, "/api/apps/*")
					appRoutes = append(appRoutes, "/ws/apps/*")
				}
				// Don't generate route paths for individual apps anymore
			}

			// Add the app-generated routes to the explicit routes
			routes := req.Routes
			if len(appRoutes) > 0 {
				// This is a set-like map to avoid duplicates
				routeSet := make(map[string]bool)

				// Add existing routes
				for _, route := range routes {
					routeSet[route] = true
				}

				// Add wildcard app routes only
				for _, route := range appRoutes {
					routeSet[route] = true
				}

				// Convert back to slice
				routes = make([]string, 0, len(routeSet))
				for route := range routeSet {
					routes = append(routes, route)
				}

				cfg.GitHub.Users[i].Routes = routes
			}

			// Update services
			cfg.GitHub.Users[i].Services = req.Services

			// Generate service routes and add them if needed
			var svcRoutes []string
			for svcType, services := range req.Services {
				for _, svc := range services {
					// Special case for wildcard
					if svc == "*" {
						// Generate routes for this service type
						svcRoutes = append(svcRoutes, fmt.Sprintf("/services/%s/*", svcType))
						svcRoutes = append(svcRoutes, fmt.Sprintf("/api/services/%s/*", svcType))
					}
					// Don't generate route paths for individual services anymore
				}
			}

			// Add the service-generated routes to the explicit routes
			if len(svcRoutes) > 0 {
				// This is a set-like map to avoid duplicates
				routeSet := make(map[string]bool)

				// Add existing routes
				for _, route := range routes {
					routeSet[route] = true
				}

				// Add wildcard service routes only
				for _, route := range svcRoutes {
					routeSet[route] = true
				}

				// Convert back to slice
				routes = make([]string, 0, len(routeSet))
				for route := range routeSet {
					routes = append(routes, route)
				}

				cfg.GitHub.Users[i].Routes = routes
			}

			userFound = true
			break
		}
	}

	if !userFound {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_routes_update_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"reason": "user_not_found"
			}`, username)),
			StatusCode: 404,
		})
		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	if err := config.SaveConfig(cfg); err != nil {
		_ = servicelogs.LogActivity(models.ActivityLog{
			Time:      time.Now(),
			RequestID: uuid.New().String(),
			IP:        c.IP(),
			Action:    "github_user_routes_update_failed",
			Method:    c.Method(),
			Route:     c.Path(),
			Parameters: json.RawMessage(fmt.Sprintf(`{
				"username": "%s",
				"reason": "save_config_failed"
			}`, username)),
			StatusCode: 500,
		})
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to save config",
		})
	}

	paramsJSON, err := json.Marshal(struct {
		Username    string              `json:"username"`
		OldRoutes   []string            `json:"old_routes"`
		NewRoutes   []string            `json:"new_routes"`
		OldApps     []string            `json:"old_apps"`
		NewApps     []string            `json:"new_apps"`
		OldServices map[string][]string `json:"old_services"`
		NewServices map[string][]string `json:"new_services"`
	}{
		Username:    username,
		OldRoutes:   oldRoutes,
		NewRoutes:   req.Routes,
		OldApps:     oldApps,
		NewApps:     req.Apps,
		OldServices: oldServices,
		NewServices: req.Services,
	})
	if err != nil {
		paramsJSON = []byte(`{}`)
	}

	// Get the user from the session instead of Locals
	sess, _ := middleware.GetStore().Get(c)
	sessionUsername := sess.Get("username")
	if sessionUsername == nil {
		sessionUsername = sess.Get("user")
	}

	// Default to "unknown" only if we couldn't get the username from session
	user := "unknown"
	if sessionUsername != nil {
		user = sessionUsername.(string)
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       user,
		Action:     "github_user_permissions_updated",
		Method:     c.Method(),
		Route:      c.Path(),
		Parameters: paramsJSON,
		StatusCode: 200,
	})

	return c.SendStatus(200)
}

// handleGetUserOptions returns available apps and services for the user management UI
func handleGetUserOptions(c *fiber.Ctx) error {
	cfg := c.Locals("config").(*config.Config)

	// Get real apps from dokku apps:list command
	realApps, err := dokku.GetDokkuApps(commandRunner)
	if err != nil {
		realApps = []string{} // If there's an error, use empty list
		internallog.LogWarning(fmt.Sprintf("Error getting app list from dokku: %v", err))
	}

	// Get all unique app names from existing users and merge with real apps
	appSet := make(map[string]bool)
	configApps := make(map[string]bool) // Track which apps are in config

	// Add real apps
	for _, app := range realApps {
		appSet[app] = true
	}

	// Add apps from user config
	for _, user := range cfg.GitHub.Users {
		for _, app := range user.Apps {
			if app != "*" { // Skip wildcard
				appSet[app] = true
				configApps[app] = true // Mark this app as coming from config
			}
		}
	}

	// Convert to slice
	apps := make([]string, 0, len(appSet))
	for app := range appSet {
		apps = append(apps, app)
	}

	// Sort apps alphabetically for consistent display
	sort.Strings(apps)

	// Get all service names by type from dokku
	services := make(map[string][]string)
	configServices := make(map[string]map[string]bool) // Track which services are in config

	// Get all available service types
	serviceTypes := dokku.GetServicePluginList()

	// Initialize empty slices and config tracking maps for each service type
	for _, svcType := range serviceTypes {
		services[svcType] = []string{}
		configServices[svcType] = make(map[string]bool)
	}

	// Get real services from dokku
	svcSets := make(map[string]map[string]bool)

	// Initialize service sets
	for _, svcType := range serviceTypes {
		svcSets[svcType] = make(map[string]bool)

		// Get services of this type
		svcList, _ := dokku.GetServiceNamesOnly(dokku.DefaultCommandRunner, svcType)

		// Add to service sets
		for _, svc := range svcList {
			svcSets[svcType][svc] = true
		}
	}

	// Add services from config
	for _, user := range cfg.GitHub.Users {
		for svcType, svcs := range user.Services {
			for _, svc := range svcs {
				if svc != "*" { // Skip wildcard
					// Initialize the service type if it doesn't exist yet
					if _, exists := svcSets[svcType]; !exists {
						svcSets[svcType] = make(map[string]bool)
					}
					if _, exists := configServices[svcType]; !exists {
						configServices[svcType] = make(map[string]bool)
					}

					svcSets[svcType][svc] = true
					configServices[svcType][svc] = true // Mark this service as coming from config
				}
			}
		}
	}

	// Convert sets to sorted lists
	for svcType, svcSet := range svcSets {
		svcList := make([]string, 0, len(svcSet))
		for svc := range svcSet {
			svcList = append(svcList, svc)
		}
		sort.Strings(svcList)
		services[svcType] = svcList
	}

	// Determine which service plugins are installed based on dokku commands
	installedServicePlugins, err := dokku.GetAvailableServicePluginList(commandRunner)
	if err != nil {
		internallog.LogWarning(fmt.Sprintf("Error getting available service plugins: %v", err))
		// Set default values if we can't get the list
		installedServicePlugins = serviceTypes
	}

	// Log the detected plugins
	pluginsJSON, _ := json.Marshal(installedServicePlugins)
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       "system",
		Action:     "detected_service_plugins",
		Parameters: json.RawMessage(fmt.Sprintf(`{"plugins":%s}`, string(pluginsJSON))),
		StatusCode: 200,
	})

	// List of services that support export/import functionality
	exportableServices := installedServicePlugins

	return c.JSON(fiber.Map{
		"apps":                    apps,
		"configApps":              configApps,
		"services":                services,
		"configServices":          configServices,
		"installedServicePlugins": installedServicePlugins,
		"exportableServices":      exportableServices,
	})
}
