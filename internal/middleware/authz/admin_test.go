package authz

import (
	"testing"
)

func TestAdminAuthChecker_CheckAccess(t *testing.T) {
	checker := NewAdminAuthChecker()

	tests := []struct {
		name string
		path string
		user User
	}{
		{"Admin user any path", "/apps", User{Username: "admin", AuthType: "admin"}},
		{"Admin user API", "/api/apps/list", User{Username: "admin", AuthType: "admin"}},
		{"Admin user services", "/services/postgres/db", User{Username: "admin", AuthType: "admin"}},
		{"Admin user settings", "/settings", User{Username: "admin", AuthType: "admin"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For admin checker, we don't actually need fiber context since it always returns true
			// But we'll pass nil for testing purposes
			result := checker.CheckAccess(nil, tt.user, tt.path)
			if !result {
				t.Errorf("AdminAuthChecker.CheckAccess() should always return true for admin, got false for path %q", tt.path)
			}
		})
	}
}
