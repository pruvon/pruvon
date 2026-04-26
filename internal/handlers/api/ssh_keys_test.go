package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/ssh"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestSshKeysAPI_RejectScopedUserAccess(t *testing.T) {
	app := newSshKeysAPIApp(t, config.User{
		Username: "operator",
		Role:     config.RoleUser,
		Routes:   []string{"/api/settings/ssh-keys", "/api/settings/ssh-keys/*"},
	})
	cookies := loginSshKeysUser(t, app, "operator", config.RoleUser)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/settings/ssh-keys"},
		{method: http.MethodPost, path: "/api/settings/ssh-keys", body: `{"name":"test","key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl user@example.com"}`},
		{method: http.MethodDelete, path: "/api/settings/ssh-keys/test"},
		{method: http.MethodPost, path: "/api/settings/ssh-keys/sync-github"},
	} {
		resp := sshKeysAPIRequest(t, app, tc.method, tc.path, tc.body, cookies)
		if resp.StatusCode != fiber.StatusForbidden {
			body := mustReadBody(t, resp)
			t.Fatalf("%s %s returned %d with body %s", tc.method, tc.path, resp.StatusCode, body)
		}
		body := mustReadBody(t, resp)
		if !strings.Contains(body, "Administrator access is required") {
			t.Fatalf("%s %s returned unexpected body %s", tc.method, tc.path, body)
		}
	}
}

func TestSshKeysAPI_AdminCanListKeys(t *testing.T) {
	app := newSshKeysAPIApp(t, config.User{Username: "admin", Role: config.RoleAdmin})
	cookies := loginSshKeysUser(t, app, "admin", config.RoleAdmin)
	originalRunner := commandRunner
	commandRunner = &dokku.MockCommandRunner{OutputMap: map[string]string{
		"dokku ssh-keys:list --format json": `[{"name":"test-key","fingerprint":"abc123","type":"ssh-ed25519"}]`,
	}}
	t.Cleanup(func() {
		commandRunner = originalRunner
	})

	resp := sshKeysAPIRequest(t, app, http.MethodGet, "/api/settings/ssh-keys", "", cookies)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/settings/ssh-keys returned %d", resp.StatusCode)
	}
	body := mustReadBody(t, resp)
	if !strings.Contains(body, "test-key") {
		t.Fatalf("expected key list in response, got %s", body)
	}
}

func TestSshKeysAPI_AdminCanDeleteKey(t *testing.T) {
	app := newSshKeysAPIApp(t, config.User{Username: "admin", Role: config.RoleAdmin})
	cookies := loginSshKeysUser(t, app, "admin", config.RoleAdmin)
	originalRunner := commandRunner
	commandRunner = &dokku.MockCommandRunner{OutputMap: map[string]string{
		"dokku ssh-keys:remove test-key": "removed",
	}}
	t.Cleanup(func() {
		commandRunner = originalRunner
	})

	resp := sshKeysAPIRequest(t, app, http.MethodDelete, "/api/settings/ssh-keys/test-key", "", cookies)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("DELETE /api/settings/ssh-keys/test-key returned %d", resp.StatusCode)
	}
	body := mustReadBody(t, resp)
	if !strings.Contains(body, `"output":"removed"`) {
		t.Fatalf("expected delete output in response, got %s", body)
	}
}

func TestSshKeysAPI_AdminCanSyncGitHub(t *testing.T) {
	app := newSshKeysAPIApp(t, config.User{Username: "admin", Role: config.RoleAdmin})
	cookies := loginSshKeysUser(t, app, "admin", config.RoleAdmin)

	resp := sshKeysAPIRequest(t, app, http.MethodPost, "/api/settings/ssh-keys/sync-github", "", cookies)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("POST /api/settings/ssh-keys/sync-github returned %d", resp.StatusCode)
	}
	body := mustReadBody(t, resp)
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("expected sync success response, got %s", body)
	}
	if !strings.Contains(body, `"github_username_missing"`) {
		t.Fatalf("expected skipped-user reason in response, got %s", body)
	}
}

func newSshKeysAPIApp(t *testing.T, sessionUser config.User) *fiber.App {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	cfg := &config.Config{Users: []config.User{{
		Username: "admin",
		Password: string(hashedPassword),
		Role:     config.RoleAdmin,
	}}}
	if sessionUser.Username != "" && sessionUser.Username != "admin" {
		cfg.Users = append(cfg.Users, sessionUser)
	}
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
	authorizedKeysPath := filepath.Join(dir, "authorized_keys")
	if err := os.WriteFile(authorizedKeysPath, nil, 0600); err != nil {
		t.Fatalf("failed to create authorized_keys: %v", err)
	}
	originalSSHAuthorizedKeysPath := ssh.AuthorizedKeysPath
	ssh.AuthorizedKeysPath = authorizedKeysPath
	t.Cleanup(func() {
		ssh.AuthorizedKeysPath = originalSSHAuthorizedKeysPath
	})

	app := fiber.New()
	registerSshKeysTestLoginRoute(app)
	app.Use(config.ConfigMiddleware(config.GetConfig()))
	app.Use(middleware.Auth())
	SetupSshKeysRoutes(app)
	return app
}

func registerSshKeysTestLoginRoute(app *fiber.App) {
	app.Get("/__test/login/:user/:role", func(c *fiber.Ctx) error {
		sess, err := middleware.GetStore().Get(c)
		if err != nil {
			return err
		}
		sess.Set("authenticated", true)
		sess.Set("user", c.Params("user"))
		sess.Set("username", c.Params("user"))
		sess.Set("role", c.Params("role"))
		sess.Set("auth_type", c.Params("role"))
		if err := sess.Save(); err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
}

func loginSshKeysUser(t *testing.T, app *fiber.App, user, role string) []*http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/__test/login/"+user+"/"+role, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("login returned %d", resp.StatusCode)
	}
	return resp.Cookies()
}

func sshKeysAPIRequest(t *testing.T, app *fiber.App, method, path, body string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}
	return resp
}

func mustReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return string(body)
}
