package web

import (
	"bytes"
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandleSettings handles the settings page
func HandleSettings(c *fiber.Ctx) error {
	// Get session data
	data := GetSessionData(c)

	// Only allow admin access
	if data["AuthType"] != config.RoleAdmin {
		return c.Redirect("/")
	}

	// Get current config values
	cfg := GetConfig()

	// Determine base URL
	baseURL := fmt.Sprintf("http://%s", cfg.Pruvon.Listen)

	// Get VHOST content for current domain
	currentDomain, err := dokku.ReadVHostFile()
	if err != nil {
		// Set error flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to read VHOST file: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
	}

	// Get cron settings
	cronMailfrom, err := dokku.ReadCronSetting("mailfrom")
	if err != nil {
		// Set error flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to read cron mailfrom setting: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
	}

	cronMailto, err := dokku.ReadCronSetting("mailto")
	if err != nil {
		// Set error flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to read cron mailto setting: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
	}

	// Get SSH Keys for the SSH Keys tab
	keys, err := dokku.GetSSHKeys(dokkuRunner)
	if err != nil {
		// Set error flash message
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to read SSH keys: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
	}

	// Add settings to the template data
	data["Title"] = "Settings"
	data["ActivePage"] = "settings"
	data["BaseURL"] = baseURL
	data["DefaultDomain"] = currentDomain
	data["CronMailfrom"] = cronMailfrom
	data["CronMailto"] = cronMailto
	data["Keys"] = keys

	// Get templates and render
	tmpl, err := templates.GetTemplate("settings/index.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Also load the user_management template to make it available
	userMgmtTmpl, err := templates.GetTemplate("settings/user_management.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error when loading user_management.html: "+err.Error())
	}

	// Also load the ssh_keys template to make it available
	sshKeysTmpl, err := templates.GetTemplate("settings/ssh_keys.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error when loading ssh_keys.html: "+err.Error())
	}

	// Add user_management template to the main template
	if t := userMgmtTmpl.Lookup("content"); t != nil {
		_, _ = tmpl.AddParseTree("user-management-content", t.Tree)
	} else {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to find user management content template")
	}

	// Add ssh_keys template to the main template
	if t := sshKeysTmpl.Lookup("content"); t != nil {
		_, _ = tmpl.AddParseTree("ssh-keys-content", t.Tree)
	} else {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to find ssh keys content template")
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", data); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleSaveDomainSettings handles saving domain settings
func HandleSaveDomainSettings(c *fiber.Ctx) error {
	// Get session data
	data := GetSessionData(c)

	// Only allow admin access
	if data["AuthType"] != config.RoleAdmin {
		return c.Redirect("/")
	}

	// Get form values
	defaultDomain := c.FormValue("default_domain")

	// Write to VHOST file
	if defaultDomain != "" {
		err := dokku.WriteVHostFile(defaultDomain)
		if err != nil {
			// Set flash message for error
			sess, _ := middleware.GetStore().Get(c)
			sess.Set("flash_message", fmt.Sprintf("Failed to write VHOST file: %v", err))
			sess.Set("flash_type", "error")
			_ = sess.Save()
			return c.Redirect("/settings")
		}
	}

	// Redirect with success parameter
	return c.Redirect("/settings?saved=true&tab=domain")
}

// HandleSaveCronSettings handles saving cron settings
func HandleSaveCronSettings(c *fiber.Ctx) error {
	// Get session data
	data := GetSessionData(c)

	// Only allow admin access
	if data["AuthType"] != config.RoleAdmin {
		return c.Redirect("/")
	}

	// Get form values
	mailfrom := c.FormValue("mailfrom")
	mailto := c.FormValue("mailto")

	// Save mailfrom setting
	err := dokku.WriteCronSetting("mailfrom", mailfrom)
	if err != nil {
		// Set flash message for error
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to save cron mailfrom setting: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
		return c.Redirect("/settings")
	}

	// Save mailto setting
	err = dokku.WriteCronSetting("mailto", mailto)
	if err != nil {
		// Set flash message for error
		sess, _ := middleware.GetStore().Get(c)
		sess.Set("flash_message", fmt.Sprintf("Failed to save cron mailto setting: %v", err))
		sess.Set("flash_type", "error")
		_ = sess.Save()
		return c.Redirect("/settings")
	}

	// Redirect with success parameter
	return c.Redirect("/settings?saved=true&tab=cron")
}
