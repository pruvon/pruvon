package handlers

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/appdeps"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware"
	servicelogs "github.com/pruvon/pruvon/internal/services/logs"
	"github.com/pruvon/pruvon/internal/ssh"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// setupDeveloperRoutes configures developer-specific routes
func setupDeveloperRoutes(app *fiber.App) {
	// Developer route to clear template cache
	app.Get("/dev/clear-cache", handleClearTemplateCache)
}

// setupRedirectRoutes configures legacy redirect routes
func setupRedirectRoutes(app *fiber.App) {
	// Redirect old /ssh-keys route to settings with ssh-keys tab
	app.Get("/ssh-keys", func(c *fiber.Ctx) error {
		return c.Redirect("/settings?tab=ssh-keys")
	})

	// Redirect old /users route to settings with users tab
	app.Get("/users", func(c *fiber.Ctx) error {
		return c.Redirect("/settings?tab=users")
	})
}

// setupSettingsHandlerRoutes configures settings-related handler routes (not API)
func setupSettingsHandlerRoutes(app *fiber.App, deps *appdeps.Dependencies, cfg *config.Config) {
	// Handle SSH key deletion in settings page
	app.Get("/settings/ssh-keys/delete/:name", func(c *fiber.Ctx) error {
		return handleSSHKeyDelete(c, deps)
	})

	// Handle SSH key synchronization with GitHub
	app.Get("/settings/ssh-keys/sync-github", func(c *fiber.Ctx) error {
		return handleGitHubSSHKeySync(c, deps, cfg)
	})
}

// handleClearTemplateCache handles clearing of template cache
func handleClearTemplateCache(c *fiber.Ctx) error {
	templates.ClearCache()
	_ = templates.Initialize()

	// Set success flash message
	sess, _ := middleware.GetStore().Get(c)
	sess.Set("flash_message", "Template cache cleared successfully")
	sess.Set("flash_type", "success")
	_ = sess.Save()

	// Redirect to the previous page or home
	referer := c.Get("Referer")
	if referer != "" {
		return c.Redirect(referer)
	}
	return c.Redirect("/")
}

// handleSSHKeyDelete handles SSH key deletion from settings page
func handleSSHKeyDelete(c *fiber.Ctx, deps *appdeps.Dependencies) error {
	if !isAdminSettingsSession(c) {
		return c.Redirect("/")
	}

	name := c.Params("name")

	_, err := deps.DokkuRunner.RunCommand("dokku", "ssh-keys:remove", name)
	if err != nil {
		// Set error flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to delete SSH key: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
	} else {
		// Set success flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", "SSH key deleted successfully")
		sess.Set("flash_type", "success")
		_ = sess.Save()
	}

	return c.Redirect("/settings?tab=ssh-keys")
}

// handleGitHubSSHKeySync handles SSH key synchronization with GitHub
func handleGitHubSSHKeySync(c *fiber.Ctx, deps *appdeps.Dependencies, cfg *config.Config) error {
	if !isAdminSettingsSession(c) {
		return c.Redirect("/")
	}

	_ = servicelogs.LogWithParams(c, "sync_github_ssh_keys", nil)

	result, err := ssh.SyncGitHubKeys(cfg.Users, ssh.AuthorizedKeysPath, deps.DokkuRunner, nil)
	if err != nil {
		// Set error flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to read authorized keys: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
		return c.Redirect("/settings?tab=ssh-keys")
	}

	// Set success flash message
	sess, _ := middleware.GetStore().Get(c)
	sess.Set("flash_message", fmt.Sprintf("GitHub SSH sync completed. Added %d keys, removed %d keys, failed users: %d.", result.AddedKeys, result.RemovedKeys, len(result.FailedUsers)))
	sess.Set("flash_type", "success")
	_ = sess.Save()

	return c.Redirect("/settings?tab=ssh-keys")
}

func isAdminSettingsSession(c *fiber.Ctx) bool {
	sess, _ := middleware.GetStore().Get(c)
	role, _ := sess.Get("role").(string)
	if role == "" {
		role, _ = sess.Get("auth_type").(string)
	}
	return role == config.RoleAdmin
}
