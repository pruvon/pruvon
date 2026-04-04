package authz

import (
	"strings"
)

// PublicRoutes defines routes that don't require authentication
var PublicRoutes = []string{
	"/login",
	"/api/login",
	"/auth/github",
	"/auth/github/callback",
}

// AuthenticatedUserRoutes defines routes accessible to any authenticated user
var AuthenticatedUserRoutes = []string{
	"/",
	"/api/metrics",
	"/api/server/info",
	"/api/docker/stats",
}

// IsPublicRoute checks if a path is a public route
func IsPublicRoute(path string) bool {
	for _, route := range PublicRoutes {
		if path == route {
			return true
		}
	}
	return false
}

// IsAuthenticatedUserRoute checks if a path is accessible to any authenticated user
func IsAuthenticatedUserRoute(path string) bool {
	for _, route := range AuthenticatedUserRoutes {
		if path == route || (strings.HasSuffix(route, "/*") &&
			strings.HasPrefix(path, strings.TrimSuffix(route, "*"))) {
			return true
		}
	}
	return false
}

// MatchesRoutePattern checks if a path matches a route pattern
// Patterns ending with * are treated as prefix matches
func MatchesRoutePattern(path, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}

// IsPathAllowed checks if a given path matches any of the allowed routes
func IsPathAllowed(path string, allowedRoutes []string) bool {
	for _, route := range allowedRoutes {
		if MatchesRoutePattern(path, route) {
			return true
		}
	}
	return false
}

// ExtractAppName extracts the app name from various path formats
// Supports: /apps/{name}, /api/apps/{name}/*, /ws/apps/{name}/*
func ExtractAppName(path string) string {
	var appName string

	if strings.HasPrefix(path, "/apps/") {
		appParts := strings.SplitN(strings.TrimPrefix(path, "/apps/"), "/", 2)
		if len(appParts) > 0 {
			appName = appParts[0]
		}
	} else if strings.HasPrefix(path, "/api/apps/") {
		appParts := strings.SplitN(strings.TrimPrefix(path, "/api/apps/"), "/", 2)
		if len(appParts) > 0 {
			appName = appParts[0]
		}
	} else if strings.HasPrefix(path, "/ws/apps/") {
		appParts := strings.SplitN(strings.TrimPrefix(path, "/ws/apps/"), "/", 2)
		if len(appParts) > 0 {
			appName = appParts[0]
		}
	}

	return appName
}

// ExtractServiceInfo extracts service type and name from various path formats
// Supports: /services/{type}/{name}, /api/services/{type}/{name}, /ws/services/{type}/{name}
func ExtractServiceInfo(path string) (serviceType, serviceName string) {
	var pathParts []string

	if strings.HasPrefix(path, "/services/") {
		pathParts = strings.Split(strings.TrimPrefix(path, "/services/"), "/")
	} else if strings.HasPrefix(path, "/api/services/") {
		pathParts = strings.Split(strings.TrimPrefix(path, "/api/services/"), "/")
	} else if strings.HasPrefix(path, "/ws/services/") {
		pathParts = strings.Split(strings.TrimPrefix(path, "/ws/services/"), "/")
	}

	if len(pathParts) > 0 {
		serviceType = pathParts[0]
		if len(pathParts) > 1 {
			serviceName = pathParts[1]
		}
	}

	return serviceType, serviceName
}

// GenerateAppRoutes generates all necessary routes for a given app name
func GenerateAppRoutes(appName string) []string {
	return []string{
		"/apps/" + appName,            // UI access
		"/api/apps/" + appName + "/*", // API endpoints
		"/ws/apps/" + appName + "/*",  // WebSocket endpoints
	}
}

// GenerateServiceRoutes generates all necessary routes for a given service type and name
func GenerateServiceRoutes(serviceType string, serviceName string) []string {
	return []string{
		"/services/" + serviceType + "/" + serviceName,            // UI access
		"/api/services/" + serviceType + "/" + serviceName + "/*", // API endpoints
		"/ws/services/" + serviceType + "/" + serviceName + "/*",  // WebSocket endpoints
	}
}

// IsAppRelatedPath checks if the path is related to apps
func IsAppRelatedPath(path string) bool {
	return strings.HasPrefix(path, "/apps/") ||
		strings.HasPrefix(path, "/api/apps/") ||
		strings.HasPrefix(path, "/ws/apps/")
}

// IsServiceRelatedPath checks if the path is related to services
func IsServiceRelatedPath(path string) bool {
	return strings.HasPrefix(path, "/services/") ||
		strings.HasPrefix(path, "/api/services/") ||
		strings.HasPrefix(path, "/ws/services/")
}

// IsAPIRequest checks if the path is an API request
func IsAPIRequest(path string) bool {
	return strings.HasPrefix(path, "/api/")
}
