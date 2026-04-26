package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/templates"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestHandleLogin_RendersLocalOnlyForm(t *testing.T) {
	if err := templates.Initialize(); err != nil {
		t.Fatalf("templates.Initialize failed: %v", err)
	}

	app := fiber.New()
	app.Get("/login", HandleLogin)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /login failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /login returned %d", resp.StatusCode)
	}
	body := readBodyString(t, resp)
	if !strings.Contains(body, "Sign in") {
		t.Fatalf("expected local sign-in form, got %s", body)
	}
	if strings.Contains(body, "GitHub") {
		t.Fatalf("login page should not reference GitHub auth, got %s", body)
	}
}

func TestHandleLoginAPI_RejectsDisabledAndPasswordlessUsers(t *testing.T) {
	app := newLoginTestApp(t, []config.User{
		loginTestUser(t, "disabled-user", config.RoleUser, "secret", true),
		{Username: "passwordless-user", Role: config.RoleUser},
	})

	for _, tc := range []struct {
		name     string
		payload  string
		wantCode int
	}{
		{name: "disabled user", payload: `{"username":"disabled-user","password":"secret"}`, wantCode: fiber.StatusUnauthorized},
		{name: "passwordless user", payload: `{"username":"passwordless-user","password":"secret"}`, wantCode: fiber.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("POST /api/login failed: %v", err)
			}
			if resp.StatusCode != tc.wantCode {
				t.Fatalf("POST /api/login returned %d", resp.StatusCode)
			}
			if !strings.Contains(readBodyString(t, resp), "Invalid credentials") {
				t.Fatalf("expected invalid credentials response")
			}
		})
	}
}

func TestHandleLoginAPI_RejectsWhenConfigIsNotLoaded(t *testing.T) {
	originalConfig := config.GetConfig()
	config.UpdateConfig(nil)
	defer config.UpdateConfig(originalConfig)

	middleware.GetStore()
	app := fiber.New()
	app.Post("/api/login", HandleLoginAPI)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /api/login failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("POST /api/login returned %d", resp.StatusCode)
	}
	if !strings.Contains(readBodyString(t, resp), "Invalid credentials") {
		t.Fatalf("expected invalid credentials response")
	}
}

func TestUserManagementTemplate_UsesResetHelpersForSuccessfulAsyncActions(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "templates", "html", "settings", "user_management.html"))
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}
	templateSource := string(content)

	for _, expected := range []string{
		"resetUserModal()",
		"resetPasswordModal()",
		"resetDeleteModal()",
	} {
		if !strings.Contains(templateSource, expected) {
			t.Fatalf("expected template to contain %s", expected)
		}
	}

	for _, expected := range []string{
		"const isEditingUser = !!this.editingUser;",
		"this.showSuccessMessage(isEditingUser ? 'User updated successfully' : 'User added successfully');",
		"md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100",
		"@keydown.escape.window=\"handleEscape\"",
		"return user?.has_password ? 'Update Password' : 'Set Password';",
		"showCreatePassword",
		"generateUserPassword",
		"generatePasswordUpdate",
		"isEditingLastEnabledAdmin()",
		"z-[9999]",
	} {
		if !strings.Contains(templateSource, expected) {
			t.Fatalf("expected template to contain %s", expected)
		}
	}
}

func TestSettingsTemplates_KeepFlashAboveModalsAndSupportEscape(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "settings index",
			path:     filepath.Join("..", "..", "templates", "html", "settings", "index.html"),
			expected: []string{"z-[9999]"},
		},
		{
			name:     "ssh keys",
			path:     filepath.Join("..", "..", "templates", "html", "settings", "ssh_keys.html"),
			expected: []string{"@keydown.escape.window=\"handleEscape\"", "z-[9999]", "including admins"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("failed to read template: %v", err)
			}
			templateSource := string(content)
			for _, expected := range tc.expected {
				if !strings.Contains(templateSource, expected) {
					t.Fatalf("expected template to contain %s", expected)
				}
			}
		})
	}
}

func newLoginTestApp(t *testing.T, extraUsers []config.User) *fiber.App {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")
	cfg := &config.Config{Users: []config.User{loginTestUser(t, "admin", config.RoleAdmin, "secret", false)}}
	cfg.Users = append(cfg.Users, extraUsers...)

	originalConfig := config.GetConfig()
	config.UpdateConfig(cfg)
	t.Cleanup(func() {
		config.UpdateConfig(originalConfig)
	})

	if err := config.WriteConfigFile(cfgPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	if _, err := config.LoadConfig(cfgPath); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	middleware.GetStore()
	app := fiber.New()
	app.Post("/api/login", HandleLoginAPI)
	return app
}

func loginTestUser(t *testing.T, username, role, password string, disabled bool) config.User {
	t.Helper()
	user := config.User{
		Username: username,
		Role:     role,
		Disabled: disabled,
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}
		user.Password = string(hash)
	}
	return user
}

func readBodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	return string(body)
}
