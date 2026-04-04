package api

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/models"
	"github.com/pruvon/pruvon/internal/services/logs"
	"github.com/pruvon/pruvon/internal/ssh"
	"io"
	"net/http"
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
	var req models.SSHKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request data",
		})
	}

	// SSH anahtarı geçerlilik kontrolü
	if !ssh.IsValidSSHKey(req.Key) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid SSH key. Please provide a valid SSH key.",
		})
	}

	// SSH anahtar adının benzersiz olup olmadığını kontrol et
	exists, err := ssh.IsKeyNameExists(req.Name, "/home/dokku/.ssh/authorized_keys")
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
	_ = logs.LogWithParams(c, "sync_github_ssh_keys", nil)

	cfg := c.Locals("config").(*config.Config)

	existingKeys, err := ssh.ReadAuthorizedKeys("/home/dokku/.ssh/authorized_keys")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Could not read authorized keys"})
	}

	githubKeys := make(map[string]bool)

	for _, user := range cfg.GitHub.Users {
		username := user.Username
		resp, err := http.Get(fmt.Sprintf("https://github.com/%s.keys", username))
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		keys := strings.Split(string(body), "\n")
		for i, key := range keys {
			if key == "" {
				continue
			}

			keyParts := strings.Fields(key)
			if len(keyParts) < 2 {
				continue
			}

			keyData := keyParts[1]
			githubKeys[keyData] = true

			exists := false
			for _, existingKey := range existingKeys {
				if existingKey.KeyData == keyData {
					exists = true
					break
				}
			}

			if !exists {
				keyName := fmt.Sprintf("%s-%d", username, i+1)
				tmpfile, err := os.CreateTemp("", "ssh-key-*.pub")
				if err != nil {
					continue
				}
				defer os.Remove(tmpfile.Name())

				if _, err := tmpfile.WriteString(key); err != nil {
					tmpfile.Close()
					continue
				}
				tmpfile.Close()

				_, _ = commandRunner.RunCommand("dokku", "ssh-keys:add", keyName, tmpfile.Name())
			}
		}
	}

	for _, existingKey := range existingKeys {
		if !githubKeys[existingKey.KeyData] {
			_, _ = commandRunner.RunCommand("dokku", "ssh-keys:remove", existingKey.Name)
		}
	}

	return c.JSON(fiber.Map{
		"success":      true,
		"synced_users": len(cfg.GitHub.Users),
	})
}

func handleGetSshKeys(c *fiber.Ctx) error {
	keys, err := dokku.GetSSHKeys(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("SSH anahtarları alınamadı: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"keys": keys,
	})
}
