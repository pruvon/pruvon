package authz

import (
	"testing"
)

func TestIsPublicRoute(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Login page", "/login", true},
		{"API login", "/api/login", true},
		{"Logout", "/logout", false},
		{"GitHub auth", "/auth/github", true},
		{"GitHub callback", "/auth/github/callback", true},
		{"Private route", "/apps", false},
		{"API route", "/api/apps/list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPublicRoute(tt.path)
			if result != tt.expected {
				t.Errorf("IsPublicRoute(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsAuthenticatedUserRoute(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Root path", "/", true},
		{"Metrics API", "/api/metrics", true},
		{"Server info API", "/api/server/info", true},
		{"Docker stats API", "/api/docker/stats", true},
		{"Audit overview API", "/api/audit/overview", true},
		{"Audit event API", "/api/audit/events/42", true},
		{"Apps page", "/apps", false},
		{"Random API", "/api/apps/list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAuthenticatedUserRoute(tt.path)
			if result != tt.expected {
				t.Errorf("IsAuthenticatedUserRoute(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestMatchesRoutePattern(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{"Exact match", "/apps", "/apps", true},
		{"No match", "/apps", "/services", false},
		{"Wildcard prefix match", "/apps/myapp", "/apps/*", true},
		{"Wildcard prefix no match", "/services/postgres", "/apps/*", false},
		{"Wildcard deep match", "/api/apps/myapp/env", "/api/apps/*", true},
		{"Non-wildcard no match", "/apps/myapp", "/apps", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesRoutePattern(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchesRoutePattern(%q, %q) = %v, want %v", tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestExtractAppName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"UI route", "/apps/myapp", "myapp"},
		{"UI route with subpath", "/apps/myapp/settings", "myapp"},
		{"API route", "/api/apps/testapp", "testapp"},
		{"API route with subpath", "/api/apps/testapp/env", "testapp"},
		{"WebSocket route", "/ws/apps/wsapp", "wsapp"},
		{"WebSocket route with subpath", "/ws/apps/wsapp/console", "wsapp"},
		{"Non-app route", "/services/postgres", ""},
		{"Root apps", "/apps", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAppName(tt.path)
			if result != tt.expected {
				t.Errorf("ExtractAppName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractServiceInfo(t *testing.T) {
	tests := []struct {
		name                string
		path                string
		expectedType        string
		expectedServiceName string
	}{
		{"UI route", "/services/postgres/mydb", "postgres", "mydb"},
		{"API route", "/api/services/mariadb/testdb", "mariadb", "testdb"},
		{"WebSocket route", "/ws/services/redis/cache", "redis", "cache"},
		{"Type only UI", "/services/postgres", "postgres", ""},
		{"Type only API", "/api/services/mongo", "mongo", ""},
		{"Non-service route", "/apps/myapp", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcType, svcName := ExtractServiceInfo(tt.path)
			if svcType != tt.expectedType || svcName != tt.expectedServiceName {
				t.Errorf("ExtractServiceInfo(%q) = (%q, %q), want (%q, %q)",
					tt.path, svcType, svcName, tt.expectedType, tt.expectedServiceName)
			}
		})
	}
}

func TestGenerateAppRoutes(t *testing.T) {
	appName := "testapp"
	routes := GenerateAppRoutes(appName)

	expected := []string{
		"/apps/testapp",
		"/api/apps/testapp/*",
		"/ws/apps/testapp/*",
	}

	if len(routes) != len(expected) {
		t.Fatalf("GenerateAppRoutes(%q) returned %d routes, want %d", appName, len(routes), len(expected))
	}

	for i, route := range routes {
		if route != expected[i] {
			t.Errorf("GenerateAppRoutes(%q)[%d] = %q, want %q", appName, i, route, expected[i])
		}
	}
}

func TestGenerateServiceRoutes(t *testing.T) {
	svcType := "postgres"
	svcName := "mydb"
	routes := GenerateServiceRoutes(svcType, svcName)

	expected := []string{
		"/services/postgres/mydb",
		"/api/services/postgres/mydb/*",
		"/ws/services/postgres/mydb/*",
	}

	if len(routes) != len(expected) {
		t.Fatalf("GenerateServiceRoutes(%q, %q) returned %d routes, want %d", svcType, svcName, len(routes), len(expected))
	}

	for i, route := range routes {
		if route != expected[i] {
			t.Errorf("GenerateServiceRoutes(%q, %q)[%d] = %q, want %q", svcType, svcName, i, route, expected[i])
		}
	}
}

func TestIsAppRelatedPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"UI app path", "/apps/myapp", true},
		{"API app path", "/api/apps/myapp/env", true},
		{"WS app path", "/ws/apps/myapp/console", true},
		{"Service path", "/services/postgres/db", false},
		{"Root path", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAppRelatedPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsAppRelatedPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsServiceRelatedPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"UI service path", "/services/postgres/db", true},
		{"API service path", "/api/services/mariadb/mydb", true},
		{"WS service path", "/ws/services/redis/cache", true},
		{"App path", "/apps/myapp", false},
		{"Root path", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsServiceRelatedPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsServiceRelatedPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsAPIRequest(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"API path", "/api/apps/list", true},
		{"API metrics", "/api/metrics", true},
		{"UI path", "/apps/myapp", false},
		{"Root path", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAPIRequest(tt.path)
			if result != tt.expected {
				t.Errorf("IsAPIRequest(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsPathAllowed(t *testing.T) {
	allowedRoutes := []string{
		"/apps/myapp",
		"/api/apps/testapp/*",
		"/services/postgres/mydb",
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Exact match", "/apps/myapp", true},
		{"Wildcard match", "/api/apps/testapp/env", true},
		{"Service exact match", "/services/postgres/mydb", true},
		{"No match", "/apps/otherapp", false},
		{"Partial match no wildcard", "/apps/myapp/settings", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPathAllowed(tt.path, allowedRoutes)
			if result != tt.expected {
				t.Errorf("IsPathAllowed(%q, allowedRoutes) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
