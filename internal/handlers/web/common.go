package web

import (
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

// GetSessionData returns common session data for templates
func GetSessionData(c *fiber.Ctx) fiber.Map {
	sess, _ := middleware.GetStore().Get(c)

	// Flash mesajlarını session'dan al
	flashMessage := sess.Get("flash_message")
	flashType := sess.Get("flash_type")

	// Flash mesajlarını kullandıktan sonra session'dan temizle
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

	return fiber.Map{
		"HideNavigation": false,
		"User":           username, // For backward compatibility
		"Username":       username, // Consistent naming
		"username":       username, // For lowercase template variables
		"AuthType":       sess.Get("auth_type"),
		"FlashMessage":   flashMessage,
		"FlashType":      flashType,
		"Version":        version, // Get version from locals
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
