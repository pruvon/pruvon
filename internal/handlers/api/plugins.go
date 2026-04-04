package api

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/services/logs"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func SetupPluginsRoutes(app *fiber.App) {
	app.Get("/api/plugins/letsencrypt/info", handleLetsencryptInfo)
	app.Post("/api/plugins/letsencrypt/email", handleLetsencryptEmail)
	app.Post("/api/plugins/letsencrypt/autorenew", handleLetsencryptAutorenew)
	app.Get("/api/plugins/letsencrypt", handleLetsencryptStatus)
	app.Get("/api/plugins/available", handlePluginsAvailable)
	app.Post("/api/plugins/install", handlePluginInstall)
	app.Get("/api/plugins/list", handlePluginsList)
}

func handleLetsencryptInfo(c *fiber.Ctx) error {
	output, err := commandRunner.RunCommand("dokku", "letsencrypt:report")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Letsencrypt report could not be retrieved: %v", err),
		})
	}
	lines := strings.Split(output, "\n")

	email := ""
	autoRenew := false
	foundApp := false

	for _, line := range lines {
		if strings.HasPrefix(line, "=====") {
			if foundApp {
				break // Exit after processing first app
			}
			foundApp = true
			continue
		}

		if !foundApp {
			continue
		}

		line = strings.TrimSpace(line)
		if strings.Contains(line, "Letsencrypt global email:") {
			email = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.Contains(line, "Letsencrypt autorenew:") {
			autoRenew = strings.TrimSpace(strings.Split(line, ":")[1]) == "true"
		}
	}

	return c.JSON(fiber.Map{
		"globalEmail": email,
		"autoRenew":   autoRenew,
	})
}

func handleLetsencryptEmail(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request",
		})
	}

	output, err := commandRunner.RunCommand("dokku", "letsencrypt:set", "--global", "email", req.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Email could not be set: %v", err),
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": output,
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleLetsencryptAutorenew(c *fiber.Ctx) error {
	output, err := commandRunner.RunCommand("dokku", "letsencrypt:cron-job", "--add")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Cron job could not be added: %v", err),
		})
	}

	if strings.Contains(strings.ToLower(output), "error") {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": output,
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleLetsencryptStatus(c *fiber.Ctx) error {
	installed, err := dokku.IsLetsencryptInstalled(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Letsencrypt durumu kontrol edilemedi: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"installed": installed,
	})
}

func handlePluginsAvailable(c *fiber.Ctx) error {
	plugins, err := dokku.GetAvailablePlugins(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Available plugins could not be retrieved: %v", err),
		})
	}
	return c.JSON(plugins)
}

func handlePluginInstall(c *fiber.Ctx) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// Plugin'i yükle
	output, err := commandRunner.RunCommand("dokku", "plugin:install", req.URL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Plugin could not be installed: %v", err),
		})
	}

	_ = logs.LogWithParams(c, "install_plugin", fiber.Map{
		"url": req.URL,
	})

	return c.JSON(fiber.Map{
		"success": !strings.Contains(output, "error"),
		"output":  output,
	})
}

func handlePluginsList(c *fiber.Ctx) error {
	plugins, err := dokku.GetPlugins(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Plugin list could not be retrieved: %v", err),
		})
	}
	return c.JSON(plugins)
}
