package authz

import (
	"testing"

	"github.com/pruvon/pruvon/internal/config"
)

func TestScopedUserAuthChecker_CheckAccess(t *testing.T) {
	testConfig := &config.Config{
		Users: []config.User{
			{
				Username: "testuser",
				Role:     config.RoleUser,
				Routes:   []string{"/apps/*"},
				Apps:     []string{"myapp", "testapp"},
				Services: map[string][]string{
					"postgres": {"db1", "db2"},
					"redis":    {"*"},
				},
			},
			{
				Username: "limiteduser",
				Role:     config.RoleUser,
				Apps:     []string{"onlyapp"},
			},
			{
				Username: "wildcarduser",
				Role:     config.RoleUser,
				Routes:   []string{"*"},
				Apps:     []string{"app1"},
				Services: map[string][]string{
					"mariadb": {"testdb"},
				},
			},
			{
				Username: "routeuser",
				Role:     config.RoleUser,
				Routes:   []string{"/apps/bar/*"},
			},
			{
				Username: "createuser",
				Role:     config.RoleUser,
				Routes:   []string{"/apps/create"},
			},
			{
				Username: "nestedrouteuser",
				Role:     config.RoleUser,
				Routes:   []string{"/apps/foo/logs"},
			},
		},
	}

	originalConfig := config.GetConfig()
	config.UpdateConfig(testConfig)
	defer config.UpdateConfig(originalConfig)

	checker := NewScopedUserAuthChecker()

	tests := []struct {
		name     string
		user     User
		path     string
		expected bool
	}{
		{"Root path allowed", User{Username: "testuser", Role: config.RoleUser}, "/", true},
		{"Root for limited user", User{Username: "limiteduser", Role: config.RoleUser}, "/", true},
		{"Route wildcard match", User{Username: "testuser", Role: config.RoleUser}, "/apps/anyapp", true},
		{"API route match", User{Username: "testuser", Role: config.RoleUser}, "/api/apps/list", true},
		{"Apps list page", User{Username: "testuser", Role: config.RoleUser}, "/apps", true},
		{"Allowed app", User{Username: "testuser", Role: config.RoleUser}, "/apps/myapp", true},
		{"Allowed app API", User{Username: "testuser", Role: config.RoleUser}, "/api/apps/myapp/env", true},
		{"Disallowed app", User{Username: "limiteduser", Role: config.RoleUser}, "/apps/myapp", false},
		{"Allowed app for limited", User{Username: "limiteduser", Role: config.RoleUser}, "/apps/onlyapp", true},
		{"Services list page", User{Username: "testuser", Role: config.RoleUser}, "/services", true},
		{"Allowed service", User{Username: "testuser", Role: config.RoleUser}, "/services/postgres/db1", true},
		{"Wildcard service", User{Username: "testuser", Role: config.RoleUser}, "/services/redis/anycache", true},
		{"Disallowed service", User{Username: "testuser", Role: config.RoleUser}, "/services/mariadb/db", false},
		{"Service API list", User{Username: "testuser", Role: config.RoleUser}, "/api/services/list", false},
		{"Wildcard apps access", User{Username: "wildcarduser", Role: config.RoleUser}, "/apps", true},
		{"Wildcard services access", User{Username: "wildcarduser", Role: config.RoleUser}, "/services", true},
		{"Wildcard docker access", User{Username: "wildcarduser", Role: config.RoleUser}, "/docker", true},
		{"Route-derived apps list access", User{Username: "routeuser", Role: config.RoleUser}, "/apps", true},
		{"Route-derived app detail access", User{Username: "routeuser", Role: config.RoleUser}, "/apps/bar", true},
		{"Route-derived app nested ui access", User{Username: "routeuser", Role: config.RoleUser}, "/apps/bar/logs", true},
		{"Route-derived app api access", User{Username: "routeuser", Role: config.RoleUser}, "/api/apps/bar/details", true},
		{"Route-derived different app denied", User{Username: "routeuser", Role: config.RoleUser}, "/apps/baz", false},
		{"Create route does not grant apps list", User{Username: "createuser", Role: config.RoleUser}, "/apps", false},
		{"Create route exact path still allowed", User{Username: "createuser", Role: config.RoleUser}, "/apps/create", true},
		{"Nested custom route does not grant apps list", User{Username: "nestedrouteuser", Role: config.RoleUser}, "/apps", false},
		{"Nested custom route does not grant parent app detail", User{Username: "nestedrouteuser", Role: config.RoleUser}, "/apps/foo", false},
		{"Nested custom route exact path still allowed", User{Username: "nestedrouteuser", Role: config.RoleUser}, "/apps/foo/logs", true},
		{"Non-existent user", User{Username: "nonexistent", Role: config.RoleUser}, "/apps", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.CheckAccess(nil, tt.user, tt.path)
			if result != tt.expected {
				t.Errorf("ScopedUserAuthChecker.CheckAccess() for user %q, path %q = %v, want %v", tt.user.Username, tt.path, result, tt.expected)
			}
		})
	}
}

func TestScopedUserAuthChecker_NoConfig(t *testing.T) {
	originalConfig := config.GetConfig()
	config.UpdateConfig(nil)
	defer config.UpdateConfig(originalConfig)

	checker := NewScopedUserAuthChecker()
	user := User{Username: "testuser", Role: config.RoleUser}
	result := checker.CheckAccess(nil, user, "/apps")

	if result {
		t.Error("ScopedUserAuthChecker.CheckAccess() should return false when config is nil")
	}
}
