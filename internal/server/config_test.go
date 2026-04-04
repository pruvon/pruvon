package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

func TestCreateDefaultConfig(t *testing.T) {
	// Create a temp directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yml")

	// Test 1: Create default config when file doesn't exist
	err := CreateDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("CreateDefaultConfig failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Read and verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read created config: %v", err)
	}

	// Unmarshal and check fields
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check admin section
	admin, ok := cfg["admin"].(map[interface{}]interface{})
	if !ok {
		t.Fatal("admin section missing or invalid")
	}
	if admin["username"] != "admin" {
		t.Errorf("Expected admin username 'admin', got %v", admin["username"])
	}

	// Check password is hashed (bcrypt hash is longer than plain "admin")
	password, ok := admin["password"].(string)
	if !ok {
		t.Fatal("admin password missing")
	}
	if len(password) <= len("admin") {
		t.Error("Password does not appear to be hashed")
	}

	// Verify password can be checked
	if err := bcrypt.CompareHashAndPassword([]byte(password), []byte("admin")); err != nil {
		t.Errorf("Generated password hash is invalid: %v", err)
	}

	// Check pruvon section
	pruvon, ok := cfg["pruvon"].(map[interface{}]interface{})
	if !ok {
		t.Fatal("pruvon section missing")
	}
	if pruvon["listen"] != "127.0.0.1:8080" {
		t.Errorf("Expected listen '127.0.0.1:8080', got %v", pruvon["listen"])
	}

	// Check backup section exists
	if _, ok := cfg["backup"]; !ok {
		t.Fatal("backup section missing")
	}

	// Test 2: Call again when file exists - should not overwrite
	originalData := string(data)
	err = CreateDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("CreateDefaultConfig failed on second call: %v", err)
	}

	newData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config after second call: %v", err)
	}

	if string(newData) != originalData {
		t.Error("Config file was modified on second call")
	}
}

func TestCreateDefaultConfig_PermissionError(t *testing.T) {
	// Test with an invalid path that causes write error
	invalidPath := "/invalid/path/config.yml"
	err := CreateDefaultConfig(invalidPath)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
	// Since the directory doesn't exist, it will try to create and fail on write
	if !strings.Contains(err.Error(), "error writing default config file") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSetupVersionMiddleware(t *testing.T) {
	app := fiber.New()
	version := "1.2.3"

	SetupVersionMiddleware(app, version)

	// Add a test route
	app.Get("/test", func(c *fiber.Ctx) error {
		v := c.Locals("version")
		if v != version {
			return c.Status(500).SendString("version mismatch")
		}
		return c.SendString("ok")
	})

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// func TestSetupUpdateCheckerMiddleware(t *testing.T) {
// 	app := fiber.New()
// 	version := "1.2.3"

// 	SetupUpdateCheckerMiddleware(app, version)

// 	// Add a test route
// 	app.Get("/test", func(c *fiber.Ctx) error {
// 		// Check that locals are set
// 		if c.Locals("updateCheckError") == nil {
// 			return c.Status(500).SendString("updateCheckError not set")
// 		}
// 		if c.Locals("updateAvailable") == nil {
// 			return c.Status(500).SendString("updateAvailable not set")
// 		}
// 		if c.Locals("latestVersion") == nil {
// 			return c.Status(500).SendString("latestVersion not set")
// 		}
// 		return c.SendString("ok")
// 	})

// 	// Test request
// 	req := httptest.NewRequest("GET", "/test", nil)
// 	resp, err := app.Test(req)
// 	if err != nil {
// 		t.Fatalf("App test failed: %v", err)
// 	}
// 	if resp.StatusCode != 200 {
// 		t.Errorf("Expected status 200, got %d", resp.StatusCode)
// 	}
// }
