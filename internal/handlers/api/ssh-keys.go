package api

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	"github.com/pruvon/pruvon/internal/services/logs"
	"github.com/pruvon/pruvon/internal/ssh"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func SetupSshKeysRoutes(app *fiber.App) {
	app.Post("/api/settings/ssh-keys", handleSshKeyAdd)
	app.Delete("/api/settings/ssh-keys/:name", handleSshKeyDelete)
	app.Post("/api/settings/ssh-keys/sync-github", handleSshKeySyncGithub)
	app.Get("/api/settings/ssh-keys", handleGetSshKeys)
}

func handleSshKeyAdd(c *fiber.Ctx) error {
	if !requireAdminSSHKeyAPI(c) {
		return nil
	}

	var req models.SSHKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// SSH key validity check
	if !ssh.IsValidSSHKey(req.Key) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid SSH key. Please provide a valid SSH key.",
		})
	}

	// Check if SSH key name is unique
	exists, err := ssh.IsKeyNameExists(req.Name, ssh.AuthorizedKeysPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("SSH keys could not be checked: %v", err),
		})
	}

	if exists {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("An SSH key named '%s' already exists. Please choose a different name.", req.Name),
		})
	}

	_ = logs.LogWithParams(c, "add_ssh_key", fiber.Map{
		"name": req.Name,
	})

	tmpfile, err := os.CreateTemp("", "ssh-key-*.pub")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Temporary file could not be created: %v", err),
		})
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(req.Key); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Key could not be written to file: %v", err),
		})
	}
	if err := tmpfile.Close(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("File could not be closed: %v", err),
		})
	}

	output, err := commandRunner.RunCommand("dokku", "ssh-keys:add", req.Name, tmpfile.Name())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("SSH key could not be added: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": !strings.Contains(strings.ToLower(output), "error"),
		"output":  output,
	})
}

func handleSshKeyDelete(c *fiber.Ctx) error {
	if !requireAdminSSHKeyAPI(c) {
		return nil
	}

	name := c.Params("name")

	_ = logs.LogWithParams(c, "delete_ssh_key", fiber.Map{
		"name": name,
	})

	output, err := commandRunner.RunCommand("dokku", "ssh-keys:remove", name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("SSH key could not be removed: %v", err),
		})
	}
	return c.JSON(fiber.Map{"output": output})
}

func handleSshKeySyncGithub(c *fiber.Ctx) error {
	if !requireAdminSSHKeyAPI(c) {
		return nil
	}

	_ = logs.LogWithParams(c, "sync_github_ssh_keys", nil)

	cfg := c.Locals("config").(*config.Config)
	result, err := ssh.SyncGitHubKeys(cfg.Users, ssh.AuthorizedKeysPath, commandRunner, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not read authorized keys"})
	}
	return c.JSON(result)
}

func handleGetSshKeys(c *fiber.Ctx) error {
	if !requireAdminSSHKeyAPI(c) {
		return nil
	}

	keys, err := dokku.GetSSHKeys(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("SSH keys could not be retrieved: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"keys": keys,
	})
}

func requireAdminSSHKeyAPI(c *fiber.Ctx) bool {
	sess, _ := middleware.GetStore().Get(c)
	role, _ := sess.Get("role").(string)
	if role == "" {
		role, _ = sess.Get("auth_type").(string)
	}
	if role == config.RoleAdmin {
		return true
	}
	_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Administrator access is required"})
	return false
}
