package web

import (
	"bytes"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandlePlugins handles the plugins listing page
func HandlePlugins(c *fiber.Ctx) error {
	plugins, err := dokku.GetPlugins(dokkuRunner)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Plugin list could not be retrieved: "+err.Error())
	}

	tmpl, err := templates.GetTemplate("plugins.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	sessionData := GetSessionData(c)
	sessionData["Plugins"] = plugins

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleLetsencrypt handles the Let's Encrypt plugin page
func HandleLetsencrypt(c *fiber.Ctx) error {
	tmpl, err := templates.GetTemplate("plugin_letsencrypt.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}
