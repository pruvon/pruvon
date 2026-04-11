package api

import (
	"github.com/pruvon/pruvon/internal/appdeps"

	"github.com/gofiber/fiber/v2"
)

func SetupApiRoutes(app *fiber.App, deps *appdeps.Dependencies) {
	initializeDependencies(deps)

	SetupAppsRoutes(app)
	SetupAuditRoutes(app)
	SetupBackupsRoutes(app)
	SetupServiceRoutes(app)
	SetupDockerRoutes(app)
	SetupOthersRoutes(app)
	SetupPluginsRoutes(app)
	SetupSshKeysRoutes(app)
	SetupLogsRoutes(app)
	SetupUsersRoutes(app)
	SetupTemplatesRoutes(app)
	SetupActivityRoutes(app)
}
