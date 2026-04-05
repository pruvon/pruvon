package main

import (
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/handlers"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/server"

	"github.com/gofiber/fiber/v2"
)

func TestPruvonVersion(t *testing.T) {
	// Test that PruvonVersion constant is defined and not empty
	if PruvonVersion == "" {
		t.Error("PruvonVersion constant should not be empty")
	}

	// Test that version follows semantic versioning pattern (basic check)
	if len(PruvonVersion) < 5 { // Minimum "0.0.0"
		t.Errorf("PruvonVersion %q seems too short for a semantic version", PruvonVersion)
	}

	// Verify it's the expected version
	expectedVersion := "0.1.0"
	if PruvonVersion != expectedVersion {
		t.Errorf("PruvonVersion = %q, expected %q", PruvonVersion, expectedVersion)
	}
}

func TestProcessBackupCommand(t *testing.T) {
	// Test with nil config
	err := processBackupCommand("auto", nil)
	if err == nil {
		t.Error("Expected error when config is nil")
	}
	if err.Error() != "configuration is not loaded for backup operation" {
		t.Errorf("Unexpected error message: %v", err)
	}

	// Create a temp directory for backup testing
	tempDir, err := os.MkdirTemp("", "pruvon_backup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with invalid command
	cfg := &config.Config{
		Backup: &config.BackupConfig{
			BackupDir:     tempDir,
			DBTypes:       []string{"postgres"},
			DoMonthly:     1,
			KeepDailyDays: 7,
		},
	}
	err = processBackupCommand("invalid", cfg)
	if err == nil {
		t.Error("Expected error for invalid backup command")
	}
	expectedErr := "invalid backup command: invalid (valid commands: auto, daily, weekly, monthly)"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}

	// Test with valid commands (these may fail due to dokku/gzip not being available in test env, but command validation passes)
	validCommands := []string{"auto", "daily", "weekly", "monthly"}
	for _, cmd := range validCommands {
		err = processBackupCommand(cmd, cfg)
		// In test environment, backup.CheckBackupPrerequisites may fail due to missing dokku/gzip
		// But if prerequisites pass, backup.Backup may fail, but we just check that it attempts
		// The important thing is that invalid commands are rejected and valid ones proceed to prerequisites
		if err != nil && err.Error() == expectedErr {
			t.Errorf("Command %s should not be treated as invalid", cmd)
		}
	}
}

func TestMainFlags(t *testing.T) {
	// Save original args and restore after test
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Reset flag command line
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Test default flag values
	backupCmd := flag.String("backup", "", "Run backup operation")
	checkListen := flag.Bool("check-listen", false, "Validate the configured listen address is available")
	configPath := flag.String("config", "/etc/pruvon.yml", "Path to the configuration file")
	serverMode := flag.Bool("server", false, "Run in server mode")
	versionFlag := flag.Bool("version", false, "Show version information")

	// Parse empty args (defaults)
	os.Args = []string{"main"}
	flag.Parse()
	if *backupCmd != "" {
		t.Errorf("Expected backupCmd to be empty, got %q", *backupCmd)
	}
	if *checkListen != false {
		t.Errorf("Expected checkListen to be false, got %t", *checkListen)
	}
	if *configPath != "/etc/pruvon.yml" {
		t.Errorf("Expected configPath to be '/etc/pruvon.yml', got %q", *configPath)
	}
	if *serverMode != false {
		t.Errorf("Expected serverMode to be false, got %t", *serverMode)
	}
	if *versionFlag != false {
		t.Errorf("Expected versionFlag to be false, got %t", *versionFlag)
	}

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	backupCmd2 := flag.String("backup", "", "Run backup operation")
	checkListen2 := flag.Bool("check-listen", false, "Validate the configured listen address is available")
	configPath2 := flag.String("config", "/etc/pruvon.yml", "Path to the configuration file")
	serverMode2 := flag.Bool("server", false, "Run in server mode")
	versionFlag2 := flag.Bool("version", false, "Show version information")

	// Test with custom args
	os.Args = []string{"main", "-backup", "auto", "-check-listen", "-config", "/tmp/config.yml", "-server", "-version"}
	flag.Parse()
	if *backupCmd2 != "auto" {
		t.Errorf("Expected backupCmd to be 'auto', got %q", *backupCmd2)
	}
	if *checkListen2 != true {
		t.Errorf("Expected checkListen to be true, got %t", *checkListen2)
	}
	if *configPath2 != "/tmp/config.yml" {
		t.Errorf("Expected configPath to be '/tmp/config.yml', got %q", *configPath2)
	}
	if *serverMode2 != true {
		t.Errorf("Expected serverMode to be true, got %t", *serverMode2)
	}
	if *versionFlag2 != true {
		t.Errorf("Expected versionFlag to be true, got %t", *versionFlag2)
	}
}

func TestResolveListenAddr(t *testing.T) {
	t.Run("uses configured listen address", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Pruvon.Listen = "127.0.0.1:9090"

		if got := resolveListenAddr(cfg); got != "127.0.0.1:9090" {
			t.Fatalf("resolveListenAddr() = %q, want %q", got, "127.0.0.1:9090")
		}
	})

	t.Run("falls back to default listen address", func(t *testing.T) {
		if got := resolveListenAddr(&config.Config{}); got != defaultListenAddr {
			t.Fatalf("resolveListenAddr() = %q, want %q", got, defaultListenAddr)
		}

		if got := resolveListenAddr(nil); got != defaultListenAddr {
			t.Fatalf("resolveListenAddr(nil) = %q, want %q", got, defaultListenAddr)
		}
	})
}

func TestCheckListenAddress(t *testing.T) {
	t.Run("accepts available address", func(t *testing.T) {
		if err := checkListenAddress("127.0.0.1:0"); err != nil {
			t.Fatalf("checkListenAddress returned error for available address: %v", err)
		}
	})

	t.Run("rejects occupied address", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to reserve test port: %v", err)
		}
		defer listener.Close()

		if err := checkListenAddress(listener.Addr().String()); err == nil {
			t.Fatal("expected error for occupied address")
		}
	})
}

func TestMainComposition_StaticRoutesRequireAuthentication(t *testing.T) {
	originalConfig := config.GetConfig()
	cfg := &config.Config{}
	config.UpdateConfig(cfg)
	defer config.UpdateConfig(originalConfig)

	app := fiber.New()
	app.Use(config.ConfigMiddleware(cfg))
	handlers.SetupRoutes(app, cfg)
	server.SetupStaticFileHandler(app)

	unauthenticated := httptest.NewRequest(http.MethodGet, "/static/js/alpine.min.js", nil)
	unauthenticatedResponse, err := app.Test(unauthenticated)
	if err != nil {
		t.Fatalf("unauthenticated static request failed: %v", err)
	}
	if unauthenticatedResponse.StatusCode != fiber.StatusFound {
		t.Fatalf("unauthenticated static request returned %d, want %d", unauthenticatedResponse.StatusCode, fiber.StatusFound)
	}
	if unauthenticatedResponse.Header.Get("Location") != "/login" {
		t.Fatalf("unauthenticated static request redirected to %q, want %q", unauthenticatedResponse.Header.Get("Location"), "/login")
	}

	loginApp := fiber.New()
	loginApp.Get("/login", func(c *fiber.Ctx) error {
		sess, err := middleware.GetStore().Get(c)
		if err != nil {
			return err
		}
		sess.Set("authenticated", true)
		sess.Set("user", "admin")
		sess.Set("username", "admin")
		sess.Set("auth_type", "admin")
		return sess.Save()
	})
	loginResponse, err := loginApp.Test(httptest.NewRequest(http.MethodGet, "/login", nil))
	if err != nil {
		t.Fatalf("session bootstrap failed: %v", err)
	}

	authenticated := httptest.NewRequest(http.MethodGet, "/static/js/alpine.min.js", nil)
	for _, cookie := range loginResponse.Cookies() {
		authenticated.AddCookie(cookie)
	}
	authenticatedResponse, err := app.Test(authenticated)
	if err != nil {
		t.Fatalf("authenticated static request failed: %v", err)
	}
	if authenticatedResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("authenticated static request returned %d, want %d", authenticatedResponse.StatusCode, fiber.StatusOK)
	}
}
