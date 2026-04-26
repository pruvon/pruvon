package server

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pruvon/pruvon/internal/services/update"

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

	// Check users section
	users, ok := cfg["users"].([]interface{})
	if !ok {
		t.Fatal("users section missing or invalid")
	}
	if len(users) != 1 {
		t.Fatalf("expected one default user, got %d", len(users))
	}
	for _, entry := range users {
		userMap, ok := entry.(map[interface{}]interface{})
		if !ok {
			t.Fatalf("unexpected user entry: %#v", entry)
		}
		if userMap["username"] != "admin" {
			t.Fatalf("fresh default config should only contain admin, got %#v", userMap["username"])
		}
	}
	admin, ok := users[0].(map[interface{}]interface{})
	if !ok {
		t.Fatal("default user missing or invalid")
	}
	if admin["username"] != "admin" {
		t.Errorf("Expected admin username 'admin', got %v", admin["username"])
	}
	if admin["role"] != "admin" {
		t.Errorf("Expected admin role 'admin', got %v", admin["role"])
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

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Fatalf("expected config file mode 0600, got %o", fileInfo.Mode().Perm())
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

func TestSetupUpdateCheckerMiddlewareUsesCachedState(t *testing.T) {
	resetUpdateCheckState()
	originalUpdateCheckFunc := updateCheckFunc
	originalUpdateCheckTTL := updateCheckTTL
	originalUpdateCheckNow := updateCheckNow
	t.Cleanup(func() {
		updateCheckFunc = originalUpdateCheckFunc
		updateCheckTTL = originalUpdateCheckTTL
		updateCheckNow = originalUpdateCheckNow
		resetUpdateCheckState()
	})

	var callCount int32
	refreshDone := make(chan struct{}, 1)
	baseTime := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	updateCheckTTL = time.Hour
	updateCheckNow = func() time.Time { return baseTime }
	updateCheckFunc = func(currentVersion string) (update.UpdateInfo, error) {
		atomic.AddInt32(&callCount, 1)
		select {
		case refreshDone <- struct{}{}:
		default:
		}
		return update.UpdateInfo{
			CurrentVersion:  currentVersion,
			LatestVersion:   "9.9.9",
			UpdateAvailable: true,
		}, nil
	}

	app := fiber.New()
	SetupUpdateCheckerMiddleware(app, "1.2.3")
	app.Get("/test", func(c *fiber.Ctx) error {
		latestVersion, _ := c.Locals("latestVersion").(string)
		return c.SendString(fmt.Sprintf("%v|%v|%s", c.Locals("updateCheckError"), c.Locals("updateAvailable"), latestVersion))
	})

	firstReq := httptest.NewRequest("GET", "/test", nil)
	firstResp, err := app.Test(firstReq)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}
	firstBody, err := io.ReadAll(firstResp.Body)
	if err != nil {
		t.Fatalf("Failed to read first response: %v", err)
	}
	if string(firstBody) != "true|false|" {
		t.Fatalf("Unexpected first response body: %s", string(firstBody))
	}

	select {
	case <-refreshDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async update refresh")
	}

	secondReq := httptest.NewRequest("GET", "/test", nil)
	secondResp, err := app.Test(secondReq)
	if err != nil {
		t.Fatalf("App test failed on second request: %v", err)
	}
	secondBody, err := io.ReadAll(secondResp.Body)
	if err != nil {
		t.Fatalf("Failed to read second response: %v", err)
	}
	if string(secondBody) != "false|true|9.9.9" {
		t.Fatalf("Unexpected second response body: %s", string(secondBody))
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected one update check, got %d", atomic.LoadInt32(&callCount))
	}
}

func TestSetupUpdateCheckerMiddlewareSkipsAPIRequests(t *testing.T) {
	resetUpdateCheckState()
	originalUpdateCheckFunc := updateCheckFunc
	t.Cleanup(func() {
		updateCheckFunc = originalUpdateCheckFunc
		resetUpdateCheckState()
	})

	var callCount int32
	updateCheckFunc = func(currentVersion string) (update.UpdateInfo, error) {
		atomic.AddInt32(&callCount, 1)
		return update.UpdateInfo{CurrentVersion: currentVersion}, nil
	}

	app := fiber.New()
	SetupUpdateCheckerMiddleware(app, "1.2.3")
	app.Get("/api/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("App test failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&callCount) != 0 {
		t.Fatalf("expected no update checks for API path, got %d", atomic.LoadInt32(&callCount))
	}
}
