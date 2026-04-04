package config

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestLoadAndSaveConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	// Write initial YAML
	initialYAML := `
admin:
  username: admin
  password: secret
github:
  client_id: abc
  client_secret: def
pruvon:
  listen: ":8080"
server:
  port: "8080"
  host: "localhost"
backup:
  backup_dir: "/tmp/backup"
  do_weekly: 1
  do_monthly: 15
  db_types: ["postgres", "redis"]
  keep_daily_days: 7
  keep_weekly_num: 4
  keep_monthly_num: 3
`
	if err := os.WriteFile(cfgPath, []byte(initialYAML), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	// Load
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg == nil || GetConfig() == nil {
		t.Fatalf("config should be loaded and set globally")
	}
	if GetConfigPath() != cfgPath {
		t.Fatalf("expected config path %s, got %s", cfgPath, GetConfigPath())
	}
	if cfg.Server == nil || cfg.Server.Port != "8080" || cfg.Server.Host != "localhost" {
		t.Fatalf("unexpected server config: %#v", cfg.Server)
	}

	// Modify and Save
	cfg.Server.Port = "9090"
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	// Verify file content updated
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}
	if !strings.Contains(string(b), "port: \"9090\"") {
		t.Fatalf("expected saved config to contain updated port, got: %s", string(b))
	}
}
func TestConfigMiddleware_SetsLocals(t *testing.T) {
	app := fiber.New()
	cfg := &Config{Server: &ServerConfig{Port: "1234", Host: "example"}}

	app.Use(ConfigMiddleware(cfg))
	app.Get("/", func(c *fiber.Ctx) error {
		v := c.Locals("config")
		if v == nil {
			t.Errorf("config locals not set")
			return c.SendStatus(500)
		}
		got := v.(*Config)
		if got.Server == nil || got.Server.Port != "1234" {
			t.Errorf("unexpected config in locals: %#v", got.Server)
		}
		return c.SendStatus(200)
	})

	req := httptestNewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber app.Test error: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status: %d body: %s", resp.StatusCode, string(body))
	}
}

func TestUpdateConfig(t *testing.T) {
	// Create a test config
	testCfg := &Config{
		Server: &ServerConfig{
			Port: "9999",
			Host: "test.example.com",
		},
	}

	// Update the global config
	UpdateConfig(testCfg)

	// Verify it was updated
	got := GetConfig()
	if got == nil {
		t.Fatal("GetConfig returned nil after UpdateConfig")
	}
	if got.Server == nil || got.Server.Port != "9999" || got.Server.Host != "test.example.com" {
		t.Errorf("UpdateConfig did not update correctly, got: %#v", got.Server)
	}
}

// Small helper to avoid importing net/http/httptest symbols in multiple packages
func httptestNewRequest(method, target string, body *strings.Reader) *http.Request {
	if body == nil {
		body = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, target, body)
	return req
}
