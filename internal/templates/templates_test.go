package templates

import (
	"html/template"
	"testing"
	"time"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "Normal date",
			input:    time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC),
			expected: "25.12.2023 15:30",
		},
		{
			name:     "First day of year",
			input:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "01.01.2024 00:00",
		},
		{
			name:     "Midnight",
			input:    time.Date(2023, 6, 15, 23, 59, 0, 0, time.UTC),
			expected: "15.06.2023 23:59",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDate(tt.input)
			if result != tt.expected {
				t.Errorf("formatDate(%v) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJsonFunc(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected template.JS
	}{
		{
			name:     "String value",
			input:    "test string",
			expected: template.JS(`"test string"`),
		},
		{
			name:     "Number value",
			input:    42,
			expected: template.JS("42"),
		},
		{
			name:     "Boolean value",
			input:    true,
			expected: template.JS("true"),
		},
		{
			name:     "Object value",
			input:    map[string]interface{}{"key": "value", "num": 123},
			expected: template.JS(`{"key":"value","num":123}`),
		},
		{
			name:     "Array value",
			input:    []string{"item1", "item2", "item3"},
			expected: template.JS(`["item1","item2","item3"]`),
		},
		{
			name:     "Nil value",
			input:    nil,
			expected: template.JS("null"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := jsonFunc(tt.input)
			if result != tt.expected {
				t.Errorf("jsonFunc(%v) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasAppAccess(t *testing.T) {
	originalConfig := config.GetConfig()
	config.UpdateConfig(&config.Config{Users: []config.User{
		{
			Username: "normal_user",
			Role:     config.RoleUser,
			Apps:     []string{"my_app"},
		},
		{
			Username: "route_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/bar/*"},
		},
		{
			Username: "create_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/create"},
		},
		{
			Username: "nested_route_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/foo/logs"},
		},
	}})
	defer config.UpdateConfig(originalConfig)

	tests := []struct {
		name        string
		username    interface{}
		specificApp string
		authType    interface{}
		expected    bool
	}{
		{
			name:        "Admin user can access any app",
			username:    "admin_user",
			specificApp: "any_app",
			authType:    "admin",
			expected:    true,
		},
		{
			name:        "Admin user can access empty app name",
			username:    "admin_user",
			specificApp: "",
			authType:    "admin",
			expected:    true,
		},
		{
			name:        "Regular user with specific app",
			username:    "normal_user",
			specificApp: "my_app",
			authType:    "user",
			expected:    true,
		},
		{
			name:        "Nil username",
			username:    nil,
			specificApp: "app",
			authType:    "user",
			expected:    false,
		},
		{
			name:        "Nil authType",
			username:    "user",
			specificApp: "app",
			authType:    nil,
			expected:    false,
		},
		{
			name:        "Route-derived access shows apps navigation",
			username:    "route_user",
			specificApp: "",
			authType:    "user",
			expected:    true,
		},
		{
			name:        "Route-derived access shows specific app",
			username:    "route_user",
			specificApp: "bar",
			authType:    "user",
			expected:    true,
		},
		{
			name:        "Create route does not show apps navigation",
			username:    "create_user",
			specificApp: "",
			authType:    "user",
			expected:    false,
		},
		{
			name:        "Nested custom route does not show apps navigation",
			username:    "nested_route_user",
			specificApp: "",
			authType:    "user",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAppAccess(tt.username, tt.specificApp, tt.authType)
			if result != tt.expected {
				t.Errorf("hasAppAccess(%v, %q, %v) = %v, expected %v",
					tt.username, tt.specificApp, tt.authType, result, tt.expected)
			}
		})
	}
}

func TestHasRouteAccess(t *testing.T) {
	originalConfig := config.GetConfig()
	config.UpdateConfig(&config.Config{Users: []config.User{
		{
			Username: "user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/*"},
		},
		{
			Username: "wildcard_user",
			Role:     config.RoleUser,
			Routes:   []string{"*"},
		},
		{
			Username: "route_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/bar/*"},
		},
		{
			Username: "create_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/create"},
		},
	}})
	defer config.UpdateConfig(originalConfig)

	tests := []struct {
		name     string
		username interface{}
		route    string
		authType interface{}
		expected bool
	}{
		{
			name:     "Admin user can access any route",
			username: "admin_user",
			route:    "/admin/settings",
			authType: "admin",
			expected: true,
		},
		{
			name:     "Apps wildcard route shows apps navigation",
			username: "user",
			route:    "/apps",
			authType: "user",
			expected: true,
		},
		{
			name:     "Regular user can access wildcard route",
			username: "user",
			route:    "/apps/test",
			authType: "user",
			expected: true,
		},
		{
			name:     "Wildcard route also grants docker access",
			username: "wildcard_user",
			route:    "/docker",
			authType: "user",
			expected: true,
		},
		{
			name:     "Route-derived access shows apps navigation",
			username: "route_user",
			route:    "/apps",
			authType: "user",
			expected: true,
		},
		{
			name:     "Route-derived access shows app detail route",
			username: "route_user",
			route:    "/apps/bar",
			authType: "user",
			expected: true,
		},
		{
			name:     "Create route does not show apps navigation",
			username: "create_user",
			route:    "/apps",
			authType: "user",
			expected: false,
		},
		{
			name:     "Create route shows exact path",
			username: "create_user",
			route:    "/apps/create",
			authType: "user",
			expected: true,
		},
		{
			name:     "Nil user",
			username: nil,
			route:    "/apps",
			authType: "user",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRouteAccess(tt.username, tt.route, tt.authType)
			if result != tt.expected {
				t.Errorf("hasRouteAccess(%v, %q, %v) = %v, expected %v",
					tt.username, tt.route, tt.authType, result, tt.expected)
			}
		})
	}
}

func TestGetUserAllowedApps_IgnoresReservedAndNestedAppRoutes(t *testing.T) {
	originalConfig := config.GetConfig()
	config.UpdateConfig(&config.Config{Users: []config.User{
		{
			Username: "create_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/create"},
		},
		{
			Username: "nested_route_user",
			Role:     config.RoleUser,
			Routes:   []string{"/apps/foo/logs"},
		},
	}})
	defer config.UpdateConfig(originalConfig)

	allApps := []string{"foo", "bar", "create"}

	assert.Empty(t, getUserAllowedApps("create_user", "user", allApps))
	assert.Empty(t, getUserAllowedApps("nested_route_user", "user", allApps))
}

func TestInitialize(t *testing.T) {
	// Test that Initialize runs without error
	err := Initialize()
	assert.NoError(t, err, "Initialize should not return an error")

	// Test that components slice is populated (assuming embedded templates exist)
	assert.NotEmpty(t, components, "components slice should not be empty after Initialize")
}
