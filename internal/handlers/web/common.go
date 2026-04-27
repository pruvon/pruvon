package web

import (
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// GetSessionData returns common session data for templates
func GetSessionData(c *fiber.Ctx) fiber.Map {
	sess, _ := middleware.GetStore().Get(c)

	// Get flash messages from the session
	flashMessage := sess.Get("flash_message")
	flashType := sess.Get("flash_type")

	// Clear flash messages from session after reading
	if flashMessage != nil {
		sess.Delete("flash_message")
		sess.Delete("flash_type")
		_ = sess.Save()
	}

	// Get the username from session - try both possible keys
	username := sess.Get("username")
	if username == nil {
		username = sess.Get("user")
	}

	// Get version from locals which is set by middleware in main.go
	version := c.Locals("version")
	if version == nil {
		version = "Unknown" // Fallback if version is not set
	}

	updateAvailable := c.Locals("updateAvailable")
	if updateAvailable == nil {
		updateAvailable = false
	}
	updateCheckError := c.Locals("updateCheckError")
	if updateCheckError == nil {
		updateCheckError = false
	}
	latestVersion := c.Locals("latestVersion")
	if latestVersion == nil {
		latestVersion = ""
	}

	return fiber.Map{
		"HideNavigation": false,
		"User":           username,
		"Username":       username,
		"username":       username,
		"AuthType": func() interface{} {
			role := sess.Get("role")
			if role != nil {
				return role
			}
			return sess.Get("auth_type")
		}(),
		"FlashMessage":     flashMessage,
		"FlashType":        flashType,
		"Version":          version,
		"version":          version,
		"updateAvailable":  updateAvailable,
		"updateCheckError": updateCheckError,
		"latestVersion":    latestVersion,
	}
}

// LoadConfig loads the configuration from the config file
// Deprecated: This function is no longer needed as config is managed centrally by config package
func LoadConfig() error {
	// Config is already loaded by the config package
	// This function is kept for backward compatibility but does nothing
	return nil
}

// SetConfig sets the configuration directly from a config object
// Deprecated: Use config.UpdateConfig() directly instead
func SetConfig(cfg *config.Config) {
	// For backward compatibility, update the central config
	config.UpdateConfig(cfg)
}

// GetConfig returns the current configuration
// This is a convenience wrapper around config.GetConfig()
func GetConfig() *config.Config {
	return config.GetConfig()
}
