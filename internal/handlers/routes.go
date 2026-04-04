package handlers

import (
	"github.com/pruvon/pruvon/internal/appdeps"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/handlers/api"
	"github.com/pruvon/pruvon/internal/handlers/web"
	"github.com/pruvon/pruvon/internal/handlers/ws"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all routes for the application
func SetupRoutes(app *fiber.App, cfg *config.Config) {
	deps := appdeps.NewDependencies(cfg)
	web.InitializeDependencies(deps)

	// Note: Config is managed centrally by the config package
	// No need to set it here as handlers use config.GetConfig() directly

	// Initialize templates
	if err := templates.Initialize(); err != nil {
		panic(err)
	}

	// Setup route groups
	setupPublicRoutes(app)
	setupAuthenticatedRoutes(app, cfg, deps)
}

// setupPublicRoutes configures routes that don't require authentication
func setupPublicRoutes(app *fiber.App) {
	app.Get("/login", web.HandleLogin)
	app.Post("/api/login", web.HandleLoginAPI)
	app.Get("/auth/github", web.HandleGithubAuth)
	app.Get("/auth/github/callback", web.HandleGithubCallback)
}

// setupAuthenticatedRoutes configures routes that require authentication
func setupAuthenticatedRoutes(app *fiber.App, cfg *config.Config, deps *appdeps.Dependencies) {
	// Apply authentication middleware
	app.Use(middleware.Auth())
	app.Get("/logout", web.HandleLogout)

	// Setup API and WebSocket routes
	api.SetupApiRoutes(app, deps)
	ws.SetupWsRoutes(app, deps)

	// Setup main application routes
	setupMainRoutes(app)

	// Setup system routes
	setupSystemRoutes(app)

	// Setup settings routes
	setupSettingsRoutes(app)

	// Setup legacy redirect routes
	setupRedirectRoutes(app)

	// Setup developer routes
	setupDeveloperRoutes(app)

	// Setup settings handler routes (non-API)
	setupSettingsHandlerRoutes(app, deps, cfg)
}

// setupMainRoutes configures main application routes
func setupMainRoutes(app *fiber.App) {
	// Dashboard
	app.Get("/", web.HandleDashboard)

	// App routes
	app.Get("/apps", web.HandleApps)
	app.Get("/apps/create", web.HandleAppCreate)
	app.Get("/apps/:name", web.HandleAppDetail)

	// Plugin routes
	app.Get("/plugins", web.HandlePlugins)
	app.Get("/plugins/letsencrypt", web.HandleLetsencrypt)

	// Service routes
	app.Get("/services", web.HandleServices)
	app.Get("/services/:type/:name", web.HandleServiceDetail)
	app.Get("/services/create", web.HandleServiceCreate)
	app.Get("/backups", web.HandleBackups)
}

// setupSystemRoutes configures system-related routes
func setupSystemRoutes(app *fiber.App) {
	app.Get("/docker", web.HandleDocker)
	app.Get("/logs", web.HandleLogs)
}

// setupSettingsRoutes configures settings routes
func setupSettingsRoutes(app *fiber.App) {
	app.Get("/settings", web.HandleSettings)
	app.Post("/settings/github", web.HandleSaveGitHubSettings)
	app.Post("/settings/domain", web.HandleSaveDomainSettings)
	app.Post("/settings/cron", web.HandleSaveCronSettings)
}
