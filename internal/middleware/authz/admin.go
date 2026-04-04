package authz

import (
	"github.com/gofiber/fiber/v2"
)

// AdminAuthChecker implements AuthChecker for admin users
type AdminAuthChecker struct{}

// NewAdminAuthChecker creates a new admin authorization checker
func NewAdminAuthChecker() *AdminAuthChecker {
	return &AdminAuthChecker{}
}

// CheckAccess always returns true for admin users
// Admin users have unrestricted access to all routes
func (a *AdminAuthChecker) CheckAccess(c *fiber.Ctx, user User, path string) bool {
	return true
}
