package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pruvon/pruvon/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func TestAuth_GitHubAppPermissions_RestrictAccessByAppAcrossRouteTypes(t *testing.T) {
	app := newAuthIntegrationApp(t, githubOnlyConfig(config.GitHubUser{
		Username: "app-user",
		Apps:     []string{"foo"},
	}), func(app *fiber.App) {
		app.Get("/apps/:name", okHandler)
		app.Get("/api/apps/:name/config", okHandler)
		app.Get("/ws/apps/:name/logs", okHandler)
	})

	cookies := loginAs(t, app, "app-user", "github")

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"allowed ui app", http.MethodGet, "/apps/foo", fiber.StatusOK},
		{"allowed api app", http.MethodGet, "/api/apps/foo/config", fiber.StatusOK},
		{"allowed ws app", http.MethodGet, "/ws/apps/foo/logs", fiber.StatusOK},
		{"forbidden sibling app prefix", http.MethodGet, "/apps/foobar", fiber.StatusForbidden},
		{"forbidden ui app", http.MethodGet, "/apps/bar", fiber.StatusForbidden},
		{"forbidden api app", http.MethodGet, "/api/apps/bar/config", fiber.StatusForbidden},
		{"forbidden ws app", http.MethodGet, "/ws/apps/bar/logs", fiber.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := performRequest(t, app, tt.method, tt.path, cookies)
			if resp.StatusCode != tt.want {
				t.Fatalf("%s %s returned %d, want %d", tt.method, tt.path, resp.StatusCode, tt.want)
			}
		})
	}
}

func TestAuth_GitHubCustomRoutes_HonorExactAndWildcardPaths(t *testing.T) {
	app := newAuthIntegrationApp(t, githubOnlyConfig(config.GitHubUser{
		Username: "custom-user",
		Routes: []string{
			"/settings",
			"/api/apps/foo/nginx/*",
		},
	}), func(app *fiber.App) {
		app.Get("/settings", okHandler)
		app.Get("/settings/advanced", okHandler)
		app.Get("/docker", okHandler)
		app.Get("/api/apps/:name/nginx/custom-config-path", okHandler)
		app.Post("/api/apps/:name/nginx/custom-config-path/reset", okHandler)
	})

	cookies := loginAs(t, app, "custom-user", "github")

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"allowed exact custom ui route", http.MethodGet, "/settings", fiber.StatusOK},
		{"exact route does not allow nested path", http.MethodGet, "/settings/advanced", fiber.StatusForbidden},
		{"forbidden unrelated ui route", http.MethodGet, "/docker", fiber.StatusForbidden},
		{"allowed wildcard custom api route", http.MethodGet, "/api/apps/foo/nginx/custom-config-path", fiber.StatusOK},
		{"allowed wildcard custom api post route", http.MethodPost, "/api/apps/foo/nginx/custom-config-path/reset", fiber.StatusOK},
		{"forbidden api route for another app", http.MethodGet, "/api/apps/bar/nginx/custom-config-path", fiber.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := performRequest(t, app, tt.method, tt.path, cookies)
			if resp.StatusCode != tt.want {
				t.Fatalf("%s %s returned %d, want %d", tt.method, tt.path, resp.StatusCode, tt.want)
			}
		})
	}
}

func TestAuth_GitHubServicePermissions_RestrictAccessByServiceAcrossRouteTypes(t *testing.T) {
	app := newAuthIntegrationApp(t, githubOnlyConfig(config.GitHubUser{
		Username: "service-user",
		Services: map[string][]string{
			"postgres": {"db1"},
			"redis":    {"*"},
		},
	}), func(app *fiber.App) {
		app.Get("/services/:type/:name", okHandler)
		app.Get("/api/services/:type/:name/info", okHandler)
		app.Get("/ws/services/:type/:name/console", okHandler)
	})

	cookies := loginAs(t, app, "service-user", "github")

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"allowed ui service", http.MethodGet, "/services/postgres/db1", fiber.StatusOK},
		{"allowed api service", http.MethodGet, "/api/services/postgres/db1/info", fiber.StatusOK},
		{"allowed ws service", http.MethodGet, "/ws/services/postgres/db1/console", fiber.StatusOK},
		{"allowed wildcard service type", http.MethodGet, "/services/redis/cache", fiber.StatusOK},
		{"forbidden different service name", http.MethodGet, "/services/postgres/db2", fiber.StatusForbidden},
		{"forbidden different service type", http.MethodGet, "/services/mongo/db1", fiber.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := performRequest(t, app, tt.method, tt.path, cookies)
			if resp.StatusCode != tt.want {
				t.Fatalf("%s %s returned %d, want %d", tt.method, tt.path, resp.StatusCode, tt.want)
			}
		})
	}
}

func TestAuth_GitHubSessionForRemovedUser_IsRevokedImmediately(t *testing.T) {
	app := newAuthIntegrationApp(t, githubOnlyConfig(), func(app *fiber.App) {
		app.Get("/apps/:name", okHandler)
	})

	cookies := loginAs(t, app, "ghost-user", "github")

	firstResponse := performRequest(t, app, http.MethodGet, "/apps/foo", cookies)
	if firstResponse.StatusCode != fiber.StatusForbidden {
		t.Fatalf("first request returned %d, want %d", firstResponse.StatusCode, fiber.StatusForbidden)
	}

	secondResponse := performRequest(t, app, http.MethodGet, "/apps/foo", cookies)
	if secondResponse.StatusCode != fiber.StatusFound {
		t.Fatalf("second request returned %d, want %d", secondResponse.StatusCode, fiber.StatusFound)
	}
	if secondResponse.Header.Get("Location") != "/login" {
		t.Fatalf("second request redirected to %q, want %q", secondResponse.Header.Get("Location"), "/login")
	}
}

func newAuthIntegrationApp(t *testing.T, cfg *config.Config, registerRoutes func(*fiber.App)) *fiber.App {
	t.Helper()

	originalConfig := config.GetConfig()
	config.UpdateConfig(cfg)
	t.Cleanup(func() {
		config.UpdateConfig(originalConfig)
	})

	store = session.New()

	app := fiber.New()
	app.Get("/__test/login/:user/:type", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}

		sess.Set("authenticated", true)
		sess.Set("user", c.Params("user"))
		sess.Set("username", c.Params("user"))
		sess.Set("auth_type", c.Params("type"))
		if err := sess.Save(); err != nil {
			return err
		}

		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Use(Auth())
	registerRoutes(app)

	return app
}

func githubOnlyConfig(users ...config.GitHubUser) *config.Config {
	cfg := &config.Config{}
	cfg.GitHub.Users = users
	return cfg
}

func loginAs(t *testing.T, app *fiber.App, user, authType string) []*http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/__test/login/"+user+"/"+authType, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("login request returned %d, want %d", resp.StatusCode, fiber.StatusNoContent)
	}

	return resp.Cookies()
}

func performRequest(t *testing.T, app *fiber.App, method, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()

	var body *strings.Reader
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		body = strings.NewReader("{}")
	} else {
		body = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, body)
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

func okHandler(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}
