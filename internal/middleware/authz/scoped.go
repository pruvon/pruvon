package authz

import (
	"strings"

	"github.com/pruvon/pruvon/internal/config"

	"github.com/gofiber/fiber/v2"
)

// ScopedUserAuthChecker implements AuthChecker for non-admin scoped users.
type ScopedUserAuthChecker struct{}

// NewScopedUserAuthChecker creates a new scoped-user authorization checker.
func NewScopedUserAuthChecker() *ScopedUserAuthChecker {
	return &ScopedUserAuthChecker{}
}

// CheckAccess checks if a scoped user has access to the given path.
func (g *ScopedUserAuthChecker) CheckAccess(c *fiber.Ctx, user User, path string) bool {
	cfg := config.GetConfig()
	if cfg == nil {
		return false
	}

	scopedUser := cfg.FindUser(user.Username)
	if scopedUser == nil || scopedUser.Disabled {
		return false
	}

	if path == "/" {
		return true
	}

	if g.hasWildcardAccess(scopedUser, path) {
		return true
	}

	if g.hasRouteAccess(scopedUser, path) {
		return true
	}

	if g.hasCreateAccess(scopedUser, path) {
		return true
	}

	if g.hasAppAccess(scopedUser, path) {
		return true
	}

	if g.hasServiceAccess(scopedUser, path) {
		return true
	}

	return false
}

func (g *ScopedUserAuthChecker) hasWildcardAccess(user *config.User, path string) bool {
	for _, route := range user.Routes {
		if route == "*" || route == "/*" {
			return true
		}
	}
	return false
}

func (g *ScopedUserAuthChecker) hasRouteAccess(user *config.User, path string) bool {
	for _, route := range user.Routes {
		if strings.HasSuffix(route, "*") {
			prefix := strings.TrimSuffix(route, "*")
			if strings.HasPrefix(path, prefix) {
				if strings.HasPrefix(path, "/apps/") && strings.HasPrefix(route, "/apps/") {
					return true
				} else if !strings.HasPrefix(path, "/apps/") {
					return true
				}
			}

			if route == "/apps/*" && strings.HasPrefix(path, "/api/apps/") {
				return true
			}
		} else if route == path {
			return true
		}
	}

	for _, route := range user.Routes {
		if IsAPIRequest(path) {
			apiRoute := strings.TrimPrefix(path, "/api")
			if strings.HasPrefix(apiRoute, "/apps/") {
				appName := strings.Split(strings.TrimPrefix(apiRoute, "/apps/"), "/")[0]
				if route == "/apps/"+appName || route == "/apps/"+appName+"/*" {
					for _, app := range user.Apps {
						if app == appName || app == "*" {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

func (g *ScopedUserAuthChecker) hasAppAccess(user *config.User, path string) bool {
	if path == "/apps" {
		return userHasAnyAppAccess(user)
	}

	if IsAppRelatedPath(path) {
		appName := ExtractAppName(path)
		if appName == "" {
			return false
		}

		if userHasExplicitAppAccess(user, appName) && pathMatchesAppAccess(path, appName) {
			return true
		}

		for _, route := range user.Routes {
			if RouteGrantsApp(route, appName) && pathMatchesAppAccess(path, appName) {
				return true
			}
		}
	}

	return false
}

func userHasAnyAppAccess(user *config.User) bool {
	if len(user.Apps) > 0 {
		return true
	}

	for _, route := range user.Routes {
		if RouteGrantsAnyApp(route) {
			return true
		}
	}

	return false
}

func userHasExplicitAppAccess(user *config.User, appName string) bool {
	for _, app := range user.Apps {
		if app == appName || app == "*" {
			return true
		}
	}

	return false
}

func pathMatchesAppAccess(path, appName string) bool {
	if IsPathAllowed(path, GenerateAppRoutes(appName)) {
		return true
	}

	return strings.HasPrefix(path, "/apps/"+appName+"/")
}

func (g *ScopedUserAuthChecker) hasServiceAccess(user *config.User, path string) bool {
	if user.Services == nil {
		return false
	}

	// Allow access to service plugin metadata endpoints for any user with service access.
	// These endpoints only return installed/available plugin names, not service data.
	if path == "/api/services/installed" || path == "/api/services/available" {
		for _, svcList := range user.Services {
			if len(svcList) > 0 {
				return true
			}
		}
		return false
	}

	if path == "/services" {
		for _, svcList := range user.Services {
			if len(svcList) > 0 {
				return true
			}
		}
		return false
	}

	if IsServiceRelatedPath(path) {
		serviceType, serviceName := ExtractServiceInfo(path)
		if serviceType == "" {
			return false
		}

		svcPermissions, hasType := user.Services[serviceType]
		if !hasType {
			return false
		}

		if serviceName == "" {
			for _, svc := range svcPermissions {
				if svc == "*" {
					return true
				}
			}
			return path == "/api/services/"+serviceType
		}

		for _, svc := range svcPermissions {
			if svc == serviceName || svc == "*" {
				return true
			}
		}
	}

	return false
}

func (g *ScopedUserAuthChecker) hasCreateAccess(user *config.User, path string) bool {
	// Explicit permission flags
	if user.CanCreateApps && (path == "/apps/create" || strings.HasPrefix(path, "/ws/apps/create")) {
		return true
	}
	if user.CanCreateServices && (path == "/services/create" || strings.HasPrefix(path, "/api/services/create") || strings.HasPrefix(path, "/ws/services/create")) {
		return true
	}

	// Legacy route-based permissions (kept in sync with template helpers)
	for _, r := range user.Routes {
		if r == "*" || r == "/*" {
			return true
		}
	}

	if path == "/apps/create" || strings.HasPrefix(path, "/ws/apps/create") {
		for _, r := range user.Routes {
			if r == "/apps/create" || r == "/apps/*" {
				return true
			}
		}
	}

	if path == "/services/create" || strings.HasPrefix(path, "/api/services/create") || strings.HasPrefix(path, "/ws/services/create") {
		for _, r := range user.Routes {
			if r == "/services/create" || r == "/services/*" {
				return true
			}
		}
	}

	return false
}
