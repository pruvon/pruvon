package web

import (
	"bytes"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandleBackups handles the backups page
func HandleBackups(c *fiber.Ctx) error {
	tmpl, err := templates.GetTemplate("backups.html")
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
