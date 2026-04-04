package middleware

import (
	"bytes"
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware/authz"
	"github.com/pruvon/pruvon/internal/templates"
	"html"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/crypto/bcrypt"
)

var store = session.New()

func GetStore() *session.Store {
	return store
}

// Auth returns the authentication middleware handler
func Auth() fiber.Handler {
	// Initialize authorization checkers
	adminChecker := authz.NewAdminAuthChecker()
	githubChecker := authz.NewGitHubAuthChecker()

	return func(c *fiber.Ctx) error {
		currentPath := c.Path()

		// 1. Check if route is public
		if authz.IsPublicRoute(currentPath) {
			return c.Next()
		}

		// 2. Validate session
		sess, err := store.Get(c)
		if err != nil {
			return handleUnauthenticated(c, currentPath, "Session error, please re-login", "Could not retrieve session")
		}

		// 3. Check authentication
		auth := sess.Get("authenticated")
		if auth == nil {
			return handleUnauthenticated(c, currentPath, "Authentication required", "You need to be logged in to access this resource")
		}
		authBool, ok := auth.(bool)
		if !ok || !authBool {
			return handleUnauthenticated(c, currentPath, "Authentication required", "You need to be logged in to access this resource")
		}

		// 4. Get user information from session
		username := getUsernameFromSession(sess)
		if username == "" {
			return handleUnauthenticated(c, currentPath, "User information missing", "Could not retrieve username from session")
		}

		authType := sess.Get("auth_type")
		if authType == nil {
			return handleUnauthenticated(c, currentPath, "Authentication type missing", "Could not retrieve auth_type from session")
		}
		authTypeStr, ok := authType.(string)
		if !ok {
			return handleUnauthenticated(c, currentPath, "Authentication type invalid", "auth_type is not a string")
		}

		// 5. Check if route is accessible to any authenticated user
		if authz.IsAuthenticatedUserRoute(currentPath) {
			return c.Next()
		}

		// 6. Create user object for authorization checking
		user := authz.User{
			Username: username,
			AuthType: authTypeStr,
		}

		// 7. Delegate to appropriate authorization checker
		var checker authz.AuthChecker
		switch authTypeStr {
		case "admin":
			checker = adminChecker
		case "github":
			// Validate GitHub user is in config before proceeding
			if !validateGitHubUser(sess, username) {
				return handleAccessDenied(c, currentPath, "Access denied - User not authorized in configuration", fiber.Map{
					"detail": "Please contact administrator",
				})
			}
			checker = githubChecker
		default:
			return handleAccessDenied(c, currentPath, "Access denied - Unknown auth type", fiber.Map{
				"auth_type": authType,
			})
		}

		// 8. Check access
		if checker.CheckAccess(c, user, currentPath) {
			return c.Next()
		}

		// 9. Access denied
		return handleAccessDenied(c, currentPath, "Access denied - Route not permitted", fiber.Map{
			"user": username,
			"path": currentPath,
		})
	}
}

// getUsernameFromSession retrieves username from session with backward compatibility
func getUsernameFromSession(sess *session.Session) string {
	username := sess.Get("username")
	if username == nil {
		username = sess.Get("user")
		if username == nil {
			return ""
		}
	}
	s, ok := username.(string)
	if !ok {
		return ""
	}
	return s
}

// validateGitHubUser validates that a GitHub user exists in the configuration
func validateGitHubUser(sess *session.Session, username string) bool {
	cfg := config.GetConfig()
	if cfg == nil {
		return false
	}

	// Check if user is in config
	userFound := false
	if cfg.GitHub.Users != nil {
		for _, user := range cfg.GitHub.Users {
			if user.Username == username {
				userFound = true
				break
			}
		}
	}

	if !userFound {
		sess.Delete("authenticated")
		sess.Delete("username")
		sess.Delete("user")
		sess.Delete("auth_type")
		if err := sess.Save(); err != nil {
			// If we can't save the session deletion, destroy it entirely
			_ = sess.Destroy()
		}
	}

	return userFound
}

// handleUnauthenticated handles unauthenticated requests
func handleUnauthenticated(c *fiber.Ctx, path, errorMsg, detail string) error {
	if authz.IsAPIRequest(path) {
		return c.Status(401).JSON(fiber.Map{
			"error":  errorMsg,
			"detail": detail,
		})
	}
	return c.Redirect("/login")
}

// handleAccessDenied handles access denied responses
func handleAccessDenied(c *fiber.Ctx, path, message string, details map[string]interface{}) error {
	if authz.IsAPIRequest(path) {
		return c.Status(403).JSON(fiber.Map{
			"error":   message,
			"details": details,
		})
	}
	return renderForbiddenPage(c, message, details)
}

// renderForbiddenPage renders a 403 forbidden page
func renderForbiddenPage(c *fiber.Ctx, message string, details map[string]interface{}) error {
	// Try to load 403 template
	tmpl, err := templates.GetTemplate("error.html")
	if err != nil {
		// If template loading fails, return simple text
		return c.Status(403).SendString("403 Forbidden: " + message)
	}

	// Get user info for the template
	sess, _ := store.Get(c)

	// Setup template data
	data := fiber.Map{
		"Title":          "Access Denied",
		"Message":        message,
		"Details":        details,
		"StatusCode":     403,
		"BackURL":        "/",
		"BackText":       "Back to Dashboard",
		"HideNavigation": false,
		"User":           sess.Get("user"),
		"AuthType":       sess.Get("auth_type"),
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return c.Status(403).SendString("403 Forbidden: " + message)
	}

	return c.Status(403).Type("html").SendString(out.String())
}

func ComparePasswords(hashedPwd string, plainPwd string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(plainPwd))
	return err == nil
}

// SetFlashMessage sets a flash message in the session
func SetFlashMessage(c *fiber.Ctx, message string, messageType string) error {
	sess, err := store.Get(c)
	if err != nil {
		return err
	}

	sess.Set("flash_message", message)
	sess.Set("flash_type", messageType)
	return sess.Save()
}

// GetFlashMessage gets and clears the flash message from the session
func GetFlashMessage(c *fiber.Ctx) (string, string) {
	sess, err := store.Get(c)
	if err != nil {
		return "", ""
	}

	message := sess.Get("flash_message")
	messageType := sess.Get("flash_type")

	// Clear the flash message
	sess.Delete("flash_message")
	sess.Delete("flash_type")
	_ = sess.Save()

	if message == nil {
		return "", ""
	}

	if messageType == nil {
		return message.(string), "info"
	}

	return message.(string), messageType.(string)
}

// FlashMiddleware adds flash message to the context
func FlashMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, _ := store.Get(c)
		flashMessage := sess.Get("flash_message")
		flashType := sess.Get("flash_type")

		// Eğer flash mesajı varsa
		if flashMessage != nil {
			var safeMessage string
			if msgStr, ok := flashMessage.(string); ok {
				// HTML özel karakterlerini encode ederek güvenli hale getir
				safeMessage = html.EscapeString(msgStr)
			} else {
				safeMessage = fmt.Sprintf("%v", flashMessage)
			}

			var safeType string
			if typeStr, ok := flashType.(string); ok {
				safeType = html.EscapeString(typeStr)
			} else {
				safeType = "info"
			}

			// Flash mesajını ve tipini context locals'a ekle
			c.Locals("FlashMessage", safeMessage)
			c.Locals("FlashType", safeType)

			// Session'dan sil - sonraki request'te tekrar gösterilmemesi için
			sess.Delete("flash_message")
			sess.Delete("flash_type")
			_ = sess.Save()
		}

		return c.Next()
	}
}
