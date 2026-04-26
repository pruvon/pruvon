package config

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestConfigMiddleware(t *testing.T) {
	// Create a test config
	testConfig := &Config{}
	testConfig.Users = []User{{
		Username: "testuser",
		Password: "testpass",
		Role:     RoleAdmin,
	}}
	testConfig.Pruvon.Listen = ":8080"

	// Create a Fiber app
	app := fiber.New()

	// Apply the middleware
	app.Use(ConfigMiddleware(testConfig))

	// Create a test route that uses the config from context
	app.Get("/test", func(c *fiber.Ctx) error {
		cfg, ok := c.Locals("config").(*Config)
		if !ok {
			return c.Status(500).SendString("Config not found in context")
		}
		return c.SendString(cfg.Users[0].Username)
	})

	// Test the middleware
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "testuser" {
		t.Errorf("Expected response body 'testuser', got '%s'", string(body))
	}
}

func TestConfigMiddleware_ConfigInContext(t *testing.T) {
	// Create a test config with specific values
	testConfig := &Config{}
	testConfig.Pruvon.Listen = ":8080"
	testConfig.Users = []User{{
		Username: "test-client-user",
		Password: "hash",
		Role:     RoleUser,
		GitHub: &UserGitHub{
			Username: "test-client-id",
		},
	}}

	// Create a Fiber app
	app := fiber.New()

	// Apply the middleware
	app.Use(ConfigMiddleware(testConfig))

	// Create a test route
	app.Get("/verify", func(c *fiber.Ctx) error {
		cfg, ok := c.Locals("config").(*Config)
		if !ok {
			return c.Status(500).SendString("Config not in context")
		}

		if cfg.Pruvon.Listen != ":8080" {
			return c.Status(500).SendString("Wrong listen address")
		}

		if cfg.Users[0].GitHub == nil || cfg.Users[0].GitHub.Username != "test-client-id" {
			return c.Status(500).SendString("Wrong GitHub metadata")
		}

		return c.SendString("OK")
	})

	// Test the middleware
	req := httptest.NewRequest("GET", "/verify", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status code 200, got %d. Body: %s", resp.StatusCode, string(body))
	}
}

func TestConfigMiddleware_NextCalled(t *testing.T) {
	// Create a test config
	testConfig := &Config{}

	// Create a Fiber app
	app := fiber.New()

	// Apply the middleware
	app.Use(ConfigMiddleware(testConfig))

	// Track if the next handler was called
	handlerCalled := false

	app.Get("/next-test", func(c *fiber.Ctx) error {
		handlerCalled = true
		return c.SendString("Next called")
	})

	// Test the middleware
	req := httptest.NewRequest("GET", "/next-test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if !handlerCalled {
		t.Error("Expected handler to be called, but it wasn't")
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}
}

func TestConfigMiddleware_MultipleRoutes(t *testing.T) {
	// Create a test config
	testConfig := &Config{}
	testConfig.Users = []User{{
		Username: "admin",
		Password: "hash",
		Role:     RoleAdmin,
	}}

	// Create a Fiber app
	app := fiber.New()

	// Apply the middleware globally
	app.Use(ConfigMiddleware(testConfig))

	// Create multiple routes
	app.Get("/route1", func(c *fiber.Ctx) error {
		cfg := c.Locals("config").(*Config)
		return c.SendString(cfg.Users[0].Username)
	})

	app.Get("/route2", func(c *fiber.Ctx) error {
		cfg := c.Locals("config").(*Config)
		return c.SendString(cfg.Users[0].Username)
	})

	// Test route1
	req1 := httptest.NewRequest("GET", "/route1", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Failed to execute request to route1: %v", err)
	}
	defer resp1.Body.Close()

	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != "admin" {
		t.Errorf("Route1: Expected 'admin', got '%s'", string(body1))
	}

	// Test route2
	req2 := httptest.NewRequest("GET", "/route2", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Failed to execute request to route2: %v", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "admin" {
		t.Errorf("Route2: Expected 'admin', got '%s'", string(body2))
	}
}
