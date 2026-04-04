package middleware

import (
	"net/http"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware/authz"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

func TestComparePasswords_UsesProvidedHash(t *testing.T) {
	originalConfig := config.GetConfig()
	config.UpdateConfig(nil)
	t.Cleanup(func() {
		config.UpdateConfig(originalConfig)
	})

	// Prepare config with a bcrypt hash of "secret"
	hashed, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	if !ComparePasswords(string(hashed), "secret") {
		t.Errorf("expected password to match when using provided hash")
	}
	if ComparePasswords(string(hashed), "wrong") {
		t.Errorf("expected wrong password to fail")
	}
}

func TestFlashMessage_SetAndGet_SameRequest(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if err := SetFlashMessage(c, "hello", "info"); err != nil {
			t.Fatalf("SetFlashMessage err: %v", err)
		}
		msg, typ := GetFlashMessage(c)
		if msg != "hello" || typ != "info" {
			t.Errorf("unexpected flash: %q %q", msg, typ)
		}
		// second read should be empty
		msg2, typ2 := GetFlashMessage(c)
		if msg2 != "" || typ2 != "" {
			t.Errorf("expected flash cleared, got %q %q", msg2, typ2)
		}
		return c.SendStatus(200)
	})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("flash test failed: %v status=%v", err, resp.StatusCode)
	}
}

func TestAuth_UnauthenticatedApiGetsJSON(t *testing.T) {
	app := fiber.New()
	app.Use(Auth())
	app.Get("/api/protected", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req, _ := http.NewRequest("GET", "/api/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for unauthenticated API, got %d", resp.StatusCode)
	}
}

func TestGenerateAppRoutes(t *testing.T) {
	routes := authz.GenerateAppRoutes("myapp")
	want := []string{"/apps/myapp", "/api/apps/myapp/*", "/ws/apps/myapp/*"}
	if len(routes) != len(want) {
		t.Fatalf("unexpected routes length: %d", len(routes))
	}
	for i := range want {
		if routes[i] != want[i] {
			t.Errorf("route[%d] = %q, want %q", i, routes[i], want[i])
		}
	}
}

func TestGenerateServiceRoutes(t *testing.T) {
	routes := authz.GenerateServiceRoutes("postgres", "db1")
	want := []string{"/services/postgres/db1", "/api/services/postgres/db1/*", "/ws/services/postgres/db1/*"}
	if len(routes) != len(want) {
		t.Fatalf("unexpected routes length: %d", len(routes))
	}
	for i := range want {
		if routes[i] != want[i] {
			t.Errorf("route[%d] = %q, want %q", i, routes[i], want[i])
		}
	}
}

func TestIsPathAllowed(t *testing.T) {
	allowed := []string{"/apps/myapp", "/api/apps/myapp/*"}

	// Exact match
	if !authz.IsPathAllowed("/apps/myapp", allowed) {
		t.Errorf("expected exact path to be allowed")
	}
	if authz.IsPathAllowed("/apps/other", allowed) {
		t.Errorf("unexpected allow for different path")
	}

	// Wildcard prefix
	if !authz.IsPathAllowed("/api/apps/myapp/logs", allowed) {
		t.Errorf("expected wildcard path to be allowed")
	}
	if authz.IsPathAllowed("/api/apps/other/logs", allowed) {
		t.Errorf("unexpected allow for different app under wildcard")
	}
}
