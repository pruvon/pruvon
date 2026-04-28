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
	scopedChecker := authz.NewScopedUserAuthChecker()

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

		role, valid := syncConfiguredUser(sess, username)
		if !valid {
			return handleUnauthenticated(c, currentPath, "Authentication required", "The configured user is no longer available")
		}

		// 5. Check if route is accessible to any authenticated user
		if authz.IsAuthenticatedUserRoute(currentPath) {
			return c.Next()
		}

		// 6. Create user object for authorization checking
		user := authz.User{
			Username: username,
			Role:     role,
		}

		// 7. Delegate to appropriate authorization checker
		var checker authz.AuthChecker
		switch role {
		case config.RoleAdmin:
			checker = adminChecker
		case config.RoleUser:
			checker = scopedChecker
		default:
			return handleAccessDenied(c, currentPath, "Access denied - Unknown auth type", fiber.Map{
				"role": role,
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

func getRoleFromSession(sess *session.Session) string {
	role, _ := sess.Get("role").(string)
	if role != "" {
		return role
	}
	authType, _ := sess.Get("auth_type").(string)
	return authType
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

// syncConfiguredUser validates that the session user still exists and is enabled,
// and returns the current configured role for authorization decisions.
func syncConfiguredUser(sess *session.Session, username string) (string, bool) {
	cfg := config.GetConfig()
	if cfg == nil {
		return "", false
	}

	user := cfg.FindUser(username)
	userFound := user != nil && !user.Disabled

	if !userFound {
		sess.Delete("authenticated")
		sess.Delete("username")
		sess.Delete("user")
		sess.Delete("role")
		sess.Delete("auth_type")
		if err := sess.Save(); err != nil {
			// If we can't save the session deletion, destroy it entirely
			_ = sess.Destroy()
		}
		return "", false
	}

	configuredRole := user.Role
	if sess.Get("role") != configuredRole || sess.Get("auth_type") != configuredRole {
		sess.Set("role", configuredRole)
		sess.Set("auth_type", configuredRole)
		_ = sess.Save()
	}

	return configuredRole, true
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

	// Get username from session - try both possible keys
	username := sess.Get("username")
	if username == nil {
		username = sess.Get("user")
	}

	// Get version/update info from locals (set by version middleware)
	version := c.Locals("version")
	if version == nil {
		version = ""
	}
	updateAvailable := c.Locals("updateAvailable")
	if updateAvailable == nil {
		updateAvailable = false
	}
	updateCheckError := c.Locals("updateCheckError")
	if updateCheckError == nil {
		updateCheckError = false
	}
	latestVersion := c.Locals("latestVersion")
	if latestVersion == nil {
		latestVersion = ""
	}

	// Setup template data
	data := fiber.Map{
		"Title":            "Access Denied",
		"Message":          message,
		"Details":          details,
		"StatusCode":       403,
		"BackURL":          "/",
		"BackText":         "Back to Dashboard",
		"HideNavigation":   false,
		"User":             username,
		"Username":         username,
		"AuthType":         getRoleFromSession(sess),
		"version":          version,
		"updateAvailable":  updateAvailable,
		"updateCheckError": updateCheckError,
		"latestVersion":    latestVersion,
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "base.html", data); err != nil {
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

		// If there is a flash message
		if flashMessage != nil {
			var safeMessage string
			if msgStr, ok := flashMessage.(string); ok {
				// Encode HTML special characters to make it safe
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

			// Add flash message and type to context locals
			c.Locals("FlashMessage", safeMessage)
			c.Locals("FlashType", safeType)

			// Delete from session so it is not shown again on the next request
			sess.Delete("flash_message")
			sess.Delete("flash_type")
			_ = sess.Save()
		}

		return c.Next()
	}
}
