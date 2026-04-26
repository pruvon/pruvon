package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pruvon/pruvon/internal/appdeps"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestSettingsSSHKeyRoutes_RejectScopedUser(t *testing.T) {
	app := newSettingsHelperTestApp(t, config.User{
		Username: "operator",
		Role:     config.RoleUser,
	})
	cookies := loginSettingsHelperUser(t, app, "operator", config.RoleUser)

	for _, path := range []string{"/settings/ssh-keys/delete/test-key", "/settings/ssh-keys/sync-github"} {
		resp := routeHelperRequest(t, app, path, cookies)
		if resp.StatusCode != fiber.StatusFound {
			t.Fatalf("GET %s returned %d", path, resp.StatusCode)
		}
		if resp.Header.Get("Location") != "/" {
			t.Fatalf("GET %s redirected to %q", path, resp.Header.Get("Location"))
		}
	}
}

func TestSettingsSSHKeyRoutes_AdminCanAccess(t *testing.T) {
	app := newSettingsHelperTestApp(t, config.User{Username: "admin", Role: config.RoleAdmin})
	cookies := loginSettingsHelperUser(t, app, "admin", config.RoleAdmin)

	deleteResp := routeHelperRequest(t, app, "/settings/ssh-keys/delete/test-key", cookies)
	if deleteResp.StatusCode != fiber.StatusFound {
		t.Fatalf("GET /settings/ssh-keys/delete/test-key returned %d", deleteResp.StatusCode)
	}
	if deleteResp.Header.Get("Location") != "/settings?tab=ssh-keys" {
		t.Fatalf("delete redirect = %q", deleteResp.Header.Get("Location"))
	}

	syncResp := routeHelperRequest(t, app, "/settings/ssh-keys/sync-github", cookies)
	if syncResp.StatusCode != fiber.StatusFound {
		t.Fatalf("GET /settings/ssh-keys/sync-github returned %d", syncResp.StatusCode)
	}
	if syncResp.Header.Get("Location") != "/settings?tab=ssh-keys" {
		t.Fatalf("sync redirect = %q", syncResp.Header.Get("Location"))
	}
}

func newSettingsHelperTestApp(t *testing.T, sessionUser config.User) *fiber.App {
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

	app := fiber.New()
	registerSettingsHelperLoginRoute(app)
	app.Use(config.ConfigMiddleware(config.GetConfig()))

	deps := &appdeps.Dependencies{
		Config: cfg,
		DokkuRunner: &dokku.MockCommandRunner{OutputMap: map[string]string{
			"dokku ssh-keys:remove test-key": "removed",
		}},
	}
	setupSettingsHandlerRoutes(app, deps, config.GetConfig())
	return app
}

func registerSettingsHelperLoginRoute(app *fiber.App) {
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

func loginSettingsHelperUser(t *testing.T, app *fiber.App, user, role string) []*http.Cookie {
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

func routeHelperRequest(t *testing.T, app *fiber.App, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request %s failed: %v", path, err)
	}
	return resp
}
