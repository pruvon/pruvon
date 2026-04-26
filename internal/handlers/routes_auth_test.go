package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/pruvon/pruvon/internal/config"

	"github.com/gofiber/fiber/v2"
)

func TestSetupRoutes_LoginPageRemainsPublicAndLogoutRequiresAuthentication(t *testing.T) {
	app := newRouteProtectionTestApp(t)

	loginResponse := routeTestRequest(t, app, http.MethodGet, "/login")
	if loginResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /login returned %d, want %d", loginResponse.StatusCode, fiber.StatusOK)
	}

	logoutResponse := routeTestRequest(t, app, http.MethodGet, "/logout")
	if logoutResponse.StatusCode != fiber.StatusFound {
		t.Fatalf("GET /logout returned %d, want %d", logoutResponse.StatusCode, fiber.StatusFound)
	}
	if logoutResponse.Header.Get("Location") != "/login" {
		t.Fatalf("GET /logout redirected to %q, want %q", logoutResponse.Header.Get("Location"), "/login")
	}
}

func TestSetupRoutes_LoginPageDoesNotReferenceProtectedStaticAssets(t *testing.T) {
	app := newRouteProtectionTestApp(t)

	resp := routeTestRequest(t, app, http.MethodGet, "/login")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /login returned %d, want %d", resp.StatusCode, fiber.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read login response body: %v", err)
	}

	if strings.Contains(string(body), "/static/") {
		t.Fatalf("login page should not reference protected /static assets")
	}
}

func TestSetupRoutes_AllProtectedRoutesRequireAuthentication(t *testing.T) {
	app := newRouteProtectionTestApp(t)

	publicRoutes := map[string]bool{
		"/login":     true,
		"/api/login": true,
	}

	for _, route := range app.GetRoutes(true) {
		if publicRoutes[route.Path] {
			continue
		}

		path := materializeRoutePath(route.Path)
		name := route.Method + " " + route.Path
		t.Run(name, func(t *testing.T) {
			resp := routeTestRequest(t, app, route.Method, path)

			if strings.HasPrefix(route.Path, "/api/") {
				if resp.StatusCode != fiber.StatusUnauthorized {
					t.Fatalf("%s returned %d, want %d", name, resp.StatusCode, fiber.StatusUnauthorized)
				}
				return
			}

			if resp.StatusCode != fiber.StatusFound {
				t.Fatalf("%s returned %d, want %d", name, resp.StatusCode, fiber.StatusFound)
			}
			if resp.Header.Get("Location") != "/login" {
				t.Fatalf("%s redirected to %q, want %q", name, resp.Header.Get("Location"), "/login")
			}
		})
	}
}

func newRouteProtectionTestApp(t *testing.T) *fiber.App {
	t.Helper()

	originalConfig := config.GetConfig()
	cfg := &config.Config{}
	config.UpdateConfig(cfg)
	t.Cleanup(func() {
		config.UpdateConfig(originalConfig)
	})

	app := fiber.New()
	app.Use(config.ConfigMiddleware(cfg))
	SetupRoutes(app, cfg)

	return app
}

func routeTestRequest(t *testing.T, app *fiber.App, method, path string) *http.Response {
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

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}

	return resp
}

func materializeRoutePath(path string) string {
	parameterPattern := regexp.MustCompile(`:[A-Za-z0-9_]+`)
	materialized := parameterPattern.ReplaceAllStringFunc(path, func(segment string) string {
		switch strings.TrimPrefix(segment, ":") {
		case "type", "dbType":
			return "postgres"
		case "backupType":
			return "daily"
		case "index":
			return "0"
		case "file":
			return "backup.tar"
		case "domain":
			return "example.com"
		default:
			return "value"
		}
	})

	return strings.ReplaceAll(materialized, "*", "value")
}
