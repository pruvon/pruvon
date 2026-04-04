package authz

import (
	"github.com/pruvon/pruvon/internal/config"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// GitHubAuthChecker implements AuthChecker for GitHub authenticated users
type GitHubAuthChecker struct{}

// NewGitHubAuthChecker creates a new GitHub authorization checker
func NewGitHubAuthChecker() *GitHubAuthChecker {
	return &GitHubAuthChecker{}
}

// CheckAccess checks if a GitHub user has access to the given path
func (g *GitHubAuthChecker) CheckAccess(c *fiber.Ctx, user User, path string) bool {
	cfg := config.GetConfig()
	if cfg == nil {
		return false
	}

	// Find user in config
	var githubUser *config.GitHubUser
	for i := range cfg.GitHub.Users {
		if cfg.GitHub.Users[i].Username == user.Username {
			githubUser = &cfg.GitHub.Users[i]
			break
		}
	}

	if githubUser == nil {
		return false
	}

	// Root path - any authenticated GitHub user can access
	if path == "/" {
		return true
	}

	// Check wildcard route access
	if g.hasWildcardAccess(githubUser, path) {
		return true
	}

	// Check specific route permissions
	if g.hasRouteAccess(githubUser, path) {
		return true
	}

	// Check app permissions
	if g.hasAppAccess(githubUser, path) {
		return true
	}

	// Check service permissions
	if g.hasServiceAccess(githubUser, path) {
		return true
	}

	return false
}

// hasWildcardAccess checks if user has wildcard access that covers the path
func (g *GitHubAuthChecker) hasWildcardAccess(user *config.GitHubUser, path string) bool {
	for _, route := range user.Routes {
		if route == "*" || route == "/*" {
			// Wildcard access - allow basic endpoints
			if path == "/" ||
				strings.HasPrefix(path, "/apps") ||
				strings.HasPrefix(path, "/services") {
				return true
			}

			// API endpoints with specific checks
			if IsAPIRequest(path) {
				if strings.HasPrefix(path, "/api/apps/list") && len(user.Apps) > 0 {
					return true
				}
				if strings.HasPrefix(path, "/api/services/list") && user.Services != nil {
					return true
				}
			}
		}
	}
	return false
}

// hasRouteAccess checks if user has explicit route permission
func (g *GitHubAuthChecker) hasRouteAccess(user *config.GitHubUser, path string) bool {
	for _, route := range user.Routes {
		// Prefix match for wildcard routes
		if strings.HasSuffix(route, "*") {
			prefix := strings.TrimSuffix(route, "*")
			if strings.HasPrefix(path, prefix) {
				// Special handling for /apps/* routes
				if strings.HasPrefix(path, "/apps/") && strings.HasPrefix(route, "/apps/") {
					return true
				} else if !strings.HasPrefix(path, "/apps/") {
					return true
				}
			}

			// Handle /apps/* matching /api/apps/* patterns
			if route == "/apps/*" && strings.HasPrefix(path, "/api/apps/") {
				return true
			}
		} else if route == path {
			// Exact match
			return true
		}
	}

	// Check API request route matching
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

// hasAppAccess checks if user has access to app-related paths
func (g *GitHubAuthChecker) hasAppAccess(user *config.GitHubUser, path string) bool {
	if len(user.Apps) == 0 {
		return false
	}

	// Allow apps list page if user has any app permissions
	if path == "/apps" {
		return true
	}

	// Check app-specific paths
	if IsAppRelatedPath(path) {
		appName := ExtractAppName(path)
		if appName == "" {
			return false
		}

		// Check if user has permission for this specific app
		for _, app := range user.Apps {
			if app == appName || app == "*" {
				// Verify the path matches generated routes for this app
				allowedRoutes := GenerateAppRoutes(appName)
				if IsPathAllowed(path, allowedRoutes) {
					return true
				}
				// Also check direct app access
				if strings.HasPrefix(path, "/apps/"+appName) {
					return true
				}
			}
		}
	}

	return false
}

// hasServiceAccess checks if user has access to service-related paths
func (g *GitHubAuthChecker) hasServiceAccess(user *config.GitHubUser, path string) bool {
	if user.Services == nil {
		return false
	}

	// Allow services list page if user has any service permissions
	if path == "/services" {
		for _, svcList := range user.Services {
			if len(svcList) > 0 {
				return true
			}
		}
		return false
	}

	// Special handling for service list API endpoints
	if path == "/api/services/list" {
		return true
	}

	// Check service-specific paths
	if IsServiceRelatedPath(path) {
		serviceType, serviceName := ExtractServiceInfo(path)

		if serviceType == "" {
			return false
		}

		// Get permissions for this service type
		svcPermissions, hasType := user.Services[serviceType]
		if !hasType {
			return false
		}

		// If no specific service name (e.g., listing services of a type)
		if serviceName == "" {
			// Check for wildcard access
			for _, svc := range svcPermissions {
				if svc == "*" {
					return true
				}
			}
			// Allow type listing endpoints
			if path == "/api/services/"+serviceType {
				return true
			}
			return false
		}

		// Check specific service access
		for _, svc := range svcPermissions {
			if svc == serviceName || svc == "*" {
				return true
			}
		}
	}

	return false
}
