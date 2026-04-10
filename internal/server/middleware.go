package server

import (
	"fmt"

	"github.com/pruvon/pruvon/internal/services/update"

	"github.com/gofiber/fiber/v2"
)

// SetupVersionMiddleware sets up middleware to inject version information into templates
func SetupVersionMiddleware(app *fiber.App, version string) {
	app.Use(func(c *fiber.Ctx) error {
		// Set version explicitly for all templates
		c.Locals("version", version)
		return c.Next()
	})
}

// SetupUpdateCheckerMiddleware sets up middleware to check for updates
func SetupUpdateCheckerMiddleware(app *fiber.App, version string) {
	app.Use(func(c *fiber.Ctx) error {
		// Check for updates silently
		updateInfo, err := update.CheckForUpdates(version)
		if err != nil {
			// Silent error handling
			c.Locals("updateCheckError", true)
			c.Locals("updateAvailable", false)
			c.Locals("latestVersion", "")
		} else {
			c.Locals("updateCheckError", false)
			c.Locals("updateAvailable", updateInfo.UpdateAvailable)
			c.Locals("latestVersion", updateInfo.LatestVersion)

			// Only log if update is available
			if updateInfo.UpdateAvailable {
				fmt.Printf("Update available! Current: v%s, Latest: v%s\n",
					updateInfo.CurrentVersion, updateInfo.LatestVersion)
			}
		}
		return c.Next()
	})
}
