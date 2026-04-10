package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/pruvon/pruvon/internal/backup"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/handlers"
	"github.com/pruvon/pruvon/internal/server"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// PruvonVersion defines the current version of Pruvon.
var PruvonVersion = "0.1.4"

const defaultListenAddr = "127.0.0.1:8080"

func resolveListenAddr(cfg *config.Config) string {
	if cfg != nil && cfg.Pruvon.Listen != "" {
		return cfg.Pruvon.Listen
	}

	return defaultListenAddr
}

func checkListenAddress(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen address %s is not available: %w", addr, err)
	}

	return listener.Close()
}

func main() {
	// Define command-line flags
	backupCmd := flag.String("backup", "", "Run backup operation: auto, daily, weekly, or monthly")
	checkListen := flag.Bool("check-listen", false, "Validate the configured listen address is available")
	configPath := flag.String("config", "/etc/pruvon.yml", "Path to the configuration file")
	serverMode := flag.Bool("server", false, "Run in server mode")
	versionFlag := flag.Bool("version", false, "Show version information")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("Pruvon version %s\n", PruvonVersion)
		os.Exit(0)
	}

	// Create default config if running in server mode and config doesn't exist
	if *serverMode {
		if err := server.CreateDefaultConfig(*configPath); err != nil {
			fmt.Printf("Error creating default config: %v\n", err)
			os.Exit(1)
		}
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		if !*serverMode && !*checkListen {
			fmt.Printf("Warning: Error loading config: %v. Proceeding without server mode.\\n", err)
		} else {
			fmt.Printf("Error loading config: %v\\n", err)
			os.Exit(1)
		}
	}

	if *checkListen {
		listenAddr := resolveListenAddr(cfg)
		if err := checkListenAddress(listenAddr); err != nil {
			fmt.Printf("Listen check failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Listen check succeeded for %s\n", listenAddr)
		return
	}

	// Process backup command if specified
	if *backupCmd != "" {
		if err := processBackupCommand(*backupCmd, cfg); err != nil {
			fmt.Printf("Backup error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	app := fiber.New(fiber.Config{
		// Proxy header setting, gets real IP address using X-Forwarded-For/X-Real-IP
		ProxyHeader: "X-Forwarded-For",
		// Define trusted proxies - typically local IPs are considered trusted
		// This setting considers all local IPs and internal networks as trusted
		TrustedProxies: []string{"127.0.0.1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		// Set a large body limit for file uploads (10GB)
		BodyLimit: 10 * 1024 * 1024 * 1024,
		// Add error handler for debugging
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Status code defaults to 500 Internal Server Error
			code := fiber.StatusInternalServerError

			// Get correct status code for Fiber specific errors
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			// Display error message
			fmt.Printf("ERROR: %v\n", err)

			// Return JSON response
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Add CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization",
	}))

	// Setup middleware
	app.Use(config.ConfigMiddleware(cfg))
	server.SetupVersionMiddleware(app, PruvonVersion)
	server.SetupUpdateCheckerMiddleware(app, PruvonVersion)

	// Setup routes
	handlers.SetupRoutes(app, cfg)
	server.SetupStaticFileHandler(app)
	fmt.Println("Static files are being served from embedded binary")

	// Start server
	listenAddr := resolveListenAddr(cfg)

	fmt.Printf("Server running on %s\n", listenAddr)
	if err := app.Listen(listenAddr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// processBackupCommand handles backup operations
func processBackupCommand(cmd string, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is not loaded for backup operation")
	}
	validCommands := map[string]bool{"auto": true, "daily": true, "weekly": true, "monthly": true}
	if !validCommands[cmd] {
		return fmt.Errorf("invalid backup command: %s (valid commands: auto, daily, weekly, monthly)", cmd)
	}
	if err := backup.CheckBackupPrerequisites(cfg); err != nil {
		return fmt.Errorf("backup prerequisites check failed: %v", err)
	}
	fmt.Printf("Starting %s backup...\n", cmd)
	if err := backup.Backup(cmd, cfg); err != nil {
		return err
	}
	fmt.Println("Backup completed successfully.")
	return nil
}
