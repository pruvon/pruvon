package web

import (
	"bytes"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandleDocker handles the Docker management page
func HandleDocker(c *fiber.Ctx) error {
	tmpl, err := templates.GetTemplate("docker.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template parse error: "+err.Error())
	}

	// Get session data
	sessionData := GetSessionData(c)
	sessionData["LoadXTerm"] = true // Required for container terminals

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", sessionData); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}
