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

	initialYAML := `
users:
  - username: admin
    password: secret
    role: admin
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
	if err := os.WriteFile(cfgPath, []byte(initialYAML), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

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
	if len(cfg.Users) != 1 || cfg.Users[0].Username != "admin" || cfg.Users[0].Role != RoleAdmin {
		t.Fatalf("unexpected canonical users: %#v", cfg.Users)
	}

	cfg.Server.Port = "9090"
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}
	if !strings.Contains(string(b), "port: \"9090\"") {
		t.Fatalf("expected saved config to contain updated port, got: %s", string(b))
	}
	if strings.Contains(string(b), "admin:") || strings.Contains(string(b), "github:") {
		t.Fatalf("expected saved config to contain only canonical users, got: %s", string(b))
	}
	if !strings.Contains(string(b), "users:") {
		t.Fatalf("expected saved config to contain users section, got: %s", string(b))
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("failed to stat saved config: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected config mode 0600, got %o", info.Mode().Perm())
	}
}

func TestLoadConfig_NormalizesLegacyAuthSections(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	legacyYAML := `
admin:
  username: admin
  password: secret
github:
  users:
    - username: octo
      routes: ["/apps/*"]
      apps: ["my-app"]
      services:
        postgres: ["main-db"]
pruvon:
  listen: ":8080"
`
	if err := os.WriteFile(cfgPath, []byte(legacyYAML), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(cfg.Users) != 2 {
		t.Fatalf("expected 2 canonical users, got %d", len(cfg.Users))
	}
	admin := cfg.FindUser("admin")
	if admin == nil || admin.Role != RoleAdmin || admin.Password != "secret" {
		t.Fatalf("unexpected admin normalization: %#v", admin)
	}
	octo := cfg.FindUser("octo")
	if octo == nil {
		t.Fatal("expected migrated legacy GitHub user")
	}
	if octo.Password != "" {
		t.Fatalf("expected migrated legacy GitHub user to have empty password, got %q", octo.Password)
	}
	if octo.GitHub == nil || octo.GitHub.Username != "octo" {
		t.Fatalf("expected github metadata to be populated, got %#v", octo.GitHub)
	}
	if len(octo.Apps) != 1 || octo.Apps[0] != "my-app" {
		t.Fatalf("unexpected migrated app permissions: %#v", octo.Apps)
	}
	if svcList := octo.Services["postgres"]; len(svcList) != 1 || svcList[0] != "main-db" {
		t.Fatalf("unexpected migrated service permissions: %#v", octo.Services)
	}
}

func TestLoadConfig_UsersSectionWinsOverLegacyAuthSections(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	yamlContent := `
users:
  - username: modern-admin
    password: secret
    role: admin
admin:
  username: old-admin
  password: legacy-secret
github:
  users:
    - username: octo
pruvon:
  listen: ":8080"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(cfg.Users) != 1 {
		t.Fatalf("expected canonical users to win, got %d users", len(cfg.Users))
	}
	if cfg.Users[0].Username != "modern-admin" {
		t.Fatalf("expected canonical user to remain authoritative, got %#v", cfg.Users[0])
	}
}

func TestLoadConfig_DuplicateLegacyUsernamesFail(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	legacyOnly := `
admin:
  username: dup
  password: secret
github:
  users:
    - username: dup
`
	if err := os.WriteFile(cfgPath, []byte(legacyOnly), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	if _, err := LoadConfig(cfgPath); err == nil {
		t.Fatal("expected duplicate username error for legacy normalization")
	}
}

func TestLoadConfig_LegacyGithubOnlyConfigFailsWithoutLocalAdmin(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	legacyOnly := `
github:
  users:
    - username: octo
      routes: ["/apps/*"]
pruvon:
  listen: ":8080"
`
	if err := os.WriteFile(cfgPath, []byte(legacyOnly), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := LoadConfig(cfgPath)
	if err == nil {
		t.Fatal("expected legacy github-only config to fail without local admin")
	}
	if !strings.Contains(err.Error(), "legacy github.users configs require a local admin with a password before migration") {
		t.Fatalf("unexpected error: %v", err)
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
	testCfg := &Config{
		Users: []User{{
			Username: "admin",
			Password: "hash",
			Role:     RoleAdmin,
		}},
		Server: &ServerConfig{
			Port: "9999",
			Host: "test.example.com",
		},
	}

	UpdateConfig(testCfg)

	got := GetConfig()
	if got == nil {
		t.Fatal("GetConfig returned nil after UpdateConfig")
	}
	if got.Server == nil || got.Server.Port != "9999" || got.Server.Host != "test.example.com" {
		t.Errorf("UpdateConfig did not update correctly, got: %#v", got.Server)
	}
	if len(got.Users) != 1 || got.Users[0].Username != "admin" {
		t.Fatalf("unexpected canonical users after UpdateConfig: %#v", got.Users)
	}
}

func TestSaveConfig_UpdatesCurrentConfigInPlace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	initial := &Config{Users: []User{{Username: "admin", Password: "hash", Role: RoleAdmin}}}
	if err := WriteConfigFile(cfgPath, initial); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	currentBefore := GetConfig()
	if currentBefore != loaded {
		t.Fatal("expected loaded config to become current config")
	}

	updated := &Config{Users: []User{{Username: "operator", Password: "hash", Role: RoleAdmin}}}
	if err := SaveConfig(updated); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	currentAfter := GetConfig()
	if currentAfter != currentBefore {
		t.Fatal("expected SaveConfig to preserve current config pointer")
	}
	if currentAfter.FindUser("operator") == nil || currentAfter.FindUser("admin") != nil {
		t.Fatalf("expected current config contents to be replaced in place, got %#v", currentAfter.Users)
	}
}

func TestWriteConfigFile_FallsBackToInPlaceOverwriteWhenAtomicReplaceIsBlocked(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "pruvon.yml")

	initial := &Config{Users: []User{{Username: "admin", Password: "hash", Role: RoleAdmin}}}
	if err := WriteConfigFile(cfgPath, initial); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatalf("failed to chmod directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0700)
	})

	updated := &Config{Users: []User{{Username: "operator", Password: "hash", Role: RoleAdmin}}}
	if err := WriteConfigFile(cfgPath, updated); err != nil {
		t.Fatalf("expected fallback overwrite to succeed, got %v", err)
	}

	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read updated config: %v", err)
	}
	if !strings.Contains(string(content), "username: operator") {
		t.Fatalf("expected updated config content, got: %s", string(content))
	}
}

func TestFindUserByUsername_NilConfigReturnsNil(t *testing.T) {
	originalConfig := GetConfig()
	UpdateConfig(nil)
	defer UpdateConfig(originalConfig)

	if user := FindUserByUsername("missing"); user != nil {
		t.Fatalf("expected nil user when config is not loaded, got %#v", user)
	}
}

// Small helper to avoid importing net/http/httptest symbols in multiple packages.
func httptestNewRequest(method, target string, body *strings.Reader) *http.Request {
	if body == nil {
		body = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, target, body)
	return req
}
