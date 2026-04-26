package web

import (
	"bytes"
	"strings"

	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
)

// HandleLogin handles the login page.
func HandleLogin(c *fiber.Ctx) error {
	tmpl, err := templates.GetTemplate("login.html")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template load error: "+err.Error())
	}

	data := fiber.Map{
		"Error":          c.Query("error"),
		"HideNavigation": true,
		"User":           nil,
		"AuthType":       nil,
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", data); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "template execute error: "+err.Error())
	}

	return c.Type("html").SendString(out.String())
}

// HandleLoginAPI handles the local login endpoint for canonical users.
func HandleLoginAPI(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	if username == "" || password == "" {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err == nil {
			username = req.Username
			password = req.Password
		}
	}

	cfg := GetConfig()
	if cfg == nil {
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
		}
		return c.Redirect("/login?error=Invalid+credentials")
	}

	user := cfg.FindUser(strings.TrimSpace(username))
	if user == nil || user.Disabled || user.Password == "" || !middleware.ComparePasswords(user.Password, password) {
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
		}
		return c.Redirect("/login?error=Invalid+credentials")
	}

	sess, err := middleware.GetStore().Get(c)
	if err != nil {
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session error"})
		}
		return c.Redirect("/login?error=Session+error")
	}

	sess.Set("authenticated", true)
	sess.Set("user", user.Username)
	sess.Set("username", user.Username)
	sess.Set("role", user.Role)
	sess.Set("auth_type", user.Role)
	if err := sess.Save(); err != nil {
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not save session"})
		}
		return c.Redirect("/login?error=Could+not+save+session")
	}

	if strings.Contains(c.Get("Accept"), "application/json") {
		return c.JSON(fiber.Map{"success": true})
	}
	return c.Redirect("/")
}

// HandleLogout handles the logout endpoint.
func HandleLogout(c *fiber.Ctx) error {
	sess, err := middleware.GetStore().Get(c)
	if err == nil {
		_ = sess.Destroy()
	}
	return c.Redirect("/login")
}
