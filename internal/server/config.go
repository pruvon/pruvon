package server

import (
	"fmt"
	"os"

	"github.com/pruvon/pruvon/internal/config"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

// CreateDefaultConfig creates a default config file with bcrypt hashed admin password
// It only creates the file if it does not already exist.
// If there's an error checking the file (other than it not existing), that error is returned.
func CreateDefaultConfig(path string) error {
	// Check file status
	_, err := os.Stat(path)

	if err == nil {
		// File exists and os.Stat was successful. No need to create a default.
		return nil
	}

	// If os.Stat returned an error, check if it's specifically because the file does not exist.
	if !os.IsNotExist(err) {
		// An error occurred, and it's NOT a "file does not exist" error.
		// This could be a permission issue on the path or the file itself.
		// We should return this error and not attempt to create a default config.
		return fmt.Errorf("could not check status of config file %s: %v (will not create default)", path, err)
	}

	// At this point, os.IsNotExist(err) is true, so the file definitely doesn't exist.
	// Proceed to create the default configuration.
	fmt.Printf("Configuration file not found at %s. Creating default configuration...\n", path)

	// Generate bcrypt hashed password for admin
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Create default config
	cfg := &config.Config{}

	// Admin settings
	cfg.Admin.Username = "admin"
	cfg.Admin.Password = string(hashedPassword)

	// Pruvon settings
	cfg.Pruvon.Listen = "127.0.0.1:8080"

	// Backup settings
	cfg.Backup = &config.BackupConfig{
		BackupDir:      "/var/lib/dokku/data/pruvon-backup",
		DoWeekly:       0, // Sunday (0=Sunday in Go's time.Weekday, both 0 and 7 are valid for Sunday)
		DoMonthly:      1, // 1st day of month
		DBTypes:        []string{"postgres", "mariadb", "mongo", "redis"},
		KeepDailyDays:  7, // Keep 7 days of daily backups
		KeepWeeklyNum:  6, // Keep 6 weeks of weekly backups
		KeepMonthlyNum: 3, // Keep 3 months of monthly backups
	}

	// Convert to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling default config: %v", err)
	}

	// Write to file
	if err := os.WriteFile(path, yamlData, 0644); err != nil {
		return fmt.Errorf("error writing default config file: %v", err)
	}

	fmt.Printf("Created default config file at %s\n", path)
	return nil
}
