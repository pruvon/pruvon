package authz

import (
	"github.com/pruvon/pruvon/internal/config"
	"testing"
)

func TestGitHubAuthChecker_CheckAccess(t *testing.T) {
	// Setup test configuration
	testConfig := &config.Config{}
	testConfig.GitHub.Users = []config.GitHubUser{
		{
			Username: "testuser",
			Routes:   []string{"/apps/*"},
			Apps:     []string{"myapp", "testapp"},
			Services: map[string][]string{
				"postgres": {"db1", "db2"},
				"redis":    {"*"},
			},
		},
		{
			Username: "limiteduser",
			Routes:   []string{},
			Apps:     []string{"onlyapp"},
			Services: nil,
		},
		{
			Username: "wildcarduser",
			Routes:   []string{"*"},
			Apps:     []string{"app1"},
			Services: map[string][]string{
				"mariadb": {"testdb"},
			},
		},
	}

	// Save current config and restore after tests
	originalConfig := config.GetConfig()
	config.UpdateConfig(testConfig)
	defer config.UpdateConfig(originalConfig)

	checker := NewGitHubAuthChecker()

	tests := []struct {
		name     string
		user     User
		path     string
		expected bool
	}{
		// Root path tests
		{"Root path allowed", User{Username: "testuser", AuthType: "github"}, "/", true},
		{"Root for limited user", User{Username: "limiteduser", AuthType: "github"}, "/", true},

		// Route permission tests
		{"Route wildcard match", User{Username: "testuser", AuthType: "github"}, "/apps/anyapp", true},
		{"API route match", User{Username: "testuser", AuthType: "github"}, "/api/apps/list", true},

		// App permission tests
		{"Apps list page", User{Username: "testuser", AuthType: "github"}, "/apps", true},
		{"Allowed app", User{Username: "testuser", AuthType: "github"}, "/apps/myapp", true},
		{"Allowed app API", User{Username: "testuser", AuthType: "github"}, "/api/apps/myapp/env", true},
		{"Disallowed app", User{Username: "limiteduser", AuthType: "github"}, "/apps/myapp", false},
		{"Allowed app for limited", User{Username: "limiteduser", AuthType: "github"}, "/apps/onlyapp", true},

		// Service permission tests
		{"Services list page", User{Username: "testuser", AuthType: "github"}, "/services", true},
		{"Allowed service", User{Username: "testuser", AuthType: "github"}, "/services/postgres/db1", true},
		{"Wildcard service", User{Username: "testuser", AuthType: "github"}, "/services/redis/anycache", true},
		{"Disallowed service", User{Username: "testuser", AuthType: "github"}, "/services/mariadb/db", false},
		{"Service API list", User{Username: "testuser", AuthType: "github"}, "/api/services/list", true},

		// Wildcard user tests
		{"Wildcard apps access", User{Username: "wildcarduser", AuthType: "github"}, "/apps", true},
		{"Wildcard services access", User{Username: "wildcarduser", AuthType: "github"}, "/services", true},

		// Non-existent user
		{"Non-existent user", User{Username: "nonexistent", AuthType: "github"}, "/apps", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GitHub checker doesn't use fiber context, it only checks config
			result := checker.CheckAccess(nil, tt.user, tt.path)
			if result != tt.expected {
				t.Errorf("GitHubAuthChecker.CheckAccess() for user %q, path %q = %v, want %v",
					tt.user.Username, tt.path, result, tt.expected)
			}
		})
	}
}

func TestGitHubAuthChecker_NoConfig(t *testing.T) {
	// Test with nil config
	originalConfig := config.GetConfig()
	config.UpdateConfig(nil)
	defer config.UpdateConfig(originalConfig)

	checker := NewGitHubAuthChecker()
	user := User{Username: "testuser", AuthType: "github"}
	result := checker.CheckAccess(nil, user, "/apps")

	if result {
		t.Error("GitHubAuthChecker.CheckAccess() should return false when config is nil")
	}
}
