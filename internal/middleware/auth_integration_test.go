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

func TestAuth_ScopedAppPermissions_RestrictAccessByAppAcrossRouteTypes(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(config.User{
		Username: "app-user",
		Role:     config.RoleUser,
		Apps:     []string{"foo"},
	}), func(app *fiber.App) {
		app.Get("/apps/:name", okHandler)
		app.Get("/api/apps/:name/config", okHandler)
		app.Get("/ws/apps/:name/logs", okHandler)
	})

	cookies := loginAs(t, app, "app-user", config.RoleUser)

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

func TestAuth_RouteDerivedAppPermissions_AllowAppRoutesWithoutAppsMembership(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(config.User{
		Username: "route-user",
		Role:     config.RoleUser,
		Routes:   []string{"/apps/bar/*"},
	}), func(app *fiber.App) {
		app.Get("/apps", okHandler)
		app.Get("/apps/:name", okHandler)
		app.Get("/apps/:name/logs", okHandler)
		app.Get("/api/apps/:name/details", okHandler)
		app.Get("/ws/apps/:name/logs", okHandler)
	})

	cookies := loginAs(t, app, "route-user", config.RoleUser)

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"apps list allowed by route-derived app grant", http.MethodGet, "/apps", fiber.StatusOK},
		{"app detail allowed by route-derived app grant", http.MethodGet, "/apps/bar", fiber.StatusOK},
		{"app nested ui allowed by route-derived app grant", http.MethodGet, "/apps/bar/logs", fiber.StatusOK},
		{"app api allowed by route-derived app grant", http.MethodGet, "/api/apps/bar/details", fiber.StatusOK},
		{"app ws allowed by route-derived app grant", http.MethodGet, "/ws/apps/bar/logs", fiber.StatusOK},
		{"other app still forbidden", http.MethodGet, "/apps/baz", fiber.StatusForbidden},
		{"other app api still forbidden", http.MethodGet, "/api/apps/baz/details", fiber.StatusForbidden},
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

func TestAuth_ScopedCustomRoutes_HonorExactAndWildcardPaths(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(config.User{
		Username: "custom-user",
		Role:     config.RoleUser,
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

	cookies := loginAs(t, app, "custom-user", config.RoleUser)

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

func TestAuth_ScopedServicePermissions_RestrictAccessByServiceAcrossRouteTypes(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(config.User{
		Username: "service-user",
		Role:     config.RoleUser,
		Services: map[string][]string{
			"postgres": {"db1"},
			"redis":    {"*"},
		},
	}), func(app *fiber.App) {
		app.Get("/services/:type/:name", okHandler)
		app.Get("/api/services/:type/:name/info", okHandler)
		app.Get("/ws/services/:type/:name/console", okHandler)
	})

	cookies := loginAs(t, app, "service-user", config.RoleUser)

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

func TestAuth_ScopedServicePermissions_AllowFilteredAppsList(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(config.User{
		Username: "service-user",
		Role:     config.RoleUser,
		Services: map[string][]string{
			"postgres": {"db1"},
		},
	}), func(app *fiber.App) {
		app.Get("/api/apps/list", okHandler)
		app.Get("/api/apps/list/detailed", okHandler)
	})

	cookies := loginAs(t, app, "service-user", config.RoleUser)

	allowedResponse := performRequest(t, app, http.MethodGet, "/api/apps/list", cookies)
	if allowedResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/apps/list returned %d, want %d", allowedResponse.StatusCode, fiber.StatusOK)
	}

	deniedResponse := performRequest(t, app, http.MethodGet, "/api/apps/list/detailed", cookies)
	if deniedResponse.StatusCode != fiber.StatusForbidden {
		t.Fatalf("GET /api/apps/list/detailed returned %d, want %d", deniedResponse.StatusCode, fiber.StatusForbidden)
	}
}

func TestAuth_SessionForRemovedUser_IsRevokedImmediately(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(), func(app *fiber.App) {
		app.Get("/apps/:name", okHandler)
	})

	cookies := loginAs(t, app, "ghost-user", config.RoleUser)

	firstResponse := performRequest(t, app, http.MethodGet, "/apps/foo", cookies)
	if firstResponse.StatusCode != fiber.StatusFound {
		t.Fatalf("first request returned %d, want %d", firstResponse.StatusCode, fiber.StatusFound)
	}
	if firstResponse.Header.Get("Location") != "/login" {
		t.Fatalf("first request redirected to %q, want %q", firstResponse.Header.Get("Location"), "/login")
	}
}

func TestAuth_SessionForRemovedAdmin_IsRevokedImmediately(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(), func(app *fiber.App) {
		app.Get("/", okHandler)
	})

	cookies := loginAs(t, app, "admin", config.RoleAdmin)

	response := performRequest(t, app, http.MethodGet, "/", cookies)
	if response.StatusCode != fiber.StatusFound {
		t.Fatalf("request returned %d, want %d", response.StatusCode, fiber.StatusFound)
	}
	if response.Header.Get("Location") != "/login" {
		t.Fatalf("request redirected to %q, want %q", response.Header.Get("Location"), "/login")
	}
}

func TestAuth_DemotedAdminSessionLosesAdminAccessImmediately(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(config.User{
		Username: "admin",
		Role:     config.RoleAdmin,
	}), func(app *fiber.App) {
		app.Get("/settings", okHandler)
		app.Get("/", okHandler)
	})

	cookies := loginAs(t, app, "admin", config.RoleAdmin)

	config.UpdateConfig(scopedUsersConfig(config.User{
		Username: "admin",
		Role:     config.RoleUser,
	}))

	adminOnlyResponse := performRequest(t, app, http.MethodGet, "/settings", cookies)
	if adminOnlyResponse.StatusCode != fiber.StatusForbidden {
		t.Fatalf("admin-only request returned %d, want %d", adminOnlyResponse.StatusCode, fiber.StatusForbidden)
	}

	authenticatedResponse := performRequest(t, app, http.MethodGet, "/", cookies)
	if authenticatedResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("authenticated-user request returned %d, want %d", authenticatedResponse.StatusCode, fiber.StatusOK)
	}
}

func TestAuth_AuthenticatedDashboardRouteStillRequiresConfiguredUser(t *testing.T) {
	app := newAuthIntegrationApp(t, scopedUsersConfig(), func(app *fiber.App) {
		app.Get("/", okHandler)
	})

	cookies := loginAs(t, app, "ghost-user", config.RoleUser)

	response := performRequest(t, app, http.MethodGet, "/", cookies)
	if response.StatusCode != fiber.StatusFound {
		t.Fatalf("request returned %d, want %d", response.StatusCode, fiber.StatusFound)
	}
	if response.Header.Get("Location") != "/login" {
		t.Fatalf("request redirected to %q, want %q", response.Header.Get("Location"), "/login")
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
	app.Get("/__test/login/:user/:role", func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
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
	app.Use(Auth())
	registerRoutes(app)

	return app
}

func scopedUsersConfig(users ...config.User) *config.Config {
	return &config.Config{Users: users}
}

func loginAs(t *testing.T, app *fiber.App, user, role string) []*http.Cookie {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/__test/login/"+user+"/"+role, nil)
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
