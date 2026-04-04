package config

import (
	"github.com/gofiber/fiber/v2"
)

// ConfigMiddleware creates a middleware that uses the already loaded configuration.
func ConfigMiddleware(cfg *Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Use the provided cfg object directly
		// cfg, err := LoadConfig(configPath)
		// if err != nil {
		// 	return c.Status(500).JSON(fiber.Map{
		// 		"error": "Could not load config",
		// 	})
		// }

		// Add config to context
		c.Locals("config", cfg)

		return c.Next()
	}
}
