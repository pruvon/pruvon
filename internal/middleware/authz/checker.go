package authz

import (
	"github.com/gofiber/fiber/v2"
)

// User represents the authenticated user information
type User struct {
	Username string
	Role     string
}

// AuthChecker defines the interface for authorization checkers
type AuthChecker interface {
	// CheckAccess checks if the user has access to the given path
	// Returns true if access is allowed, false otherwise
	CheckAccess(c *fiber.Ctx, user User, path string) bool
}

// AccessDeniedHandler handles access denied responses based on request type
type AccessDeniedHandler func(c *fiber.Ctx, message string, details map[string]interface{}) error
