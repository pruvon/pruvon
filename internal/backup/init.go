package backup

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"os"
	"os/exec"
	"path/filepath"
)

// InitBackupConfig initializes the backup configuration with default values if not set
func InitBackupConfig(cfg *config.Config) error {
	if cfg.Backup == nil {
		// Set default backup configuration
		cfg.Backup = &config.BackupConfig{
			BackupDir:      "/var/lib/dokku/data/pruvon-backup",
			DoWeekly:       7, // Sunday (7=Sunday, 1=Monday in the 1-7 system)
			DoMonthly:      1, // 1st day of month
			DBTypes:        []string{"postgres", "mariadb", "mongo", "redis"},
			KeepDailyDays:  7, // Keep daily backups for 7 days
			KeepWeeklyNum:  6, // Keep last 6 weekly backups
			KeepMonthlyNum: 3, // Keep last 3 monthly backups
		}
	} else {
		// Set default values for rotation settings if not specified
		if cfg.Backup.KeepDailyDays <= 0 {
			cfg.Backup.KeepDailyDays = 7
		}
		if cfg.Backup.KeepWeeklyNum <= 0 {
			cfg.Backup.KeepWeeklyNum = 6
		}
		if cfg.Backup.KeepMonthlyNum <= 0 {
			cfg.Backup.KeepMonthlyNum = 3
		}
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(cfg.Backup.BackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Validate backup configuration
	if err := validateBackupConfig(cfg.Backup); err != nil {
		return err
	}

	return nil
}

// validateBackupConfig validates the backup configuration
func validateBackupConfig(backupCfg *config.BackupConfig) error {
	// Check if backup directory is specified
	if backupCfg.BackupDir == "" {
		return fmt.Errorf("backup directory is not specified")
	}

	// Validate DoWeekly - accept both 0 and 1-7 (0 and 7 both represent Sunday)
	if backupCfg.DoWeekly < 0 || backupCfg.DoWeekly > 7 {
		return fmt.Errorf("invalid day of week for weekly backups (must be 0-7, where 0 and 7 both represent Sunday)")
	}

	// Validate DoMonthly
	if backupCfg.DoMonthly < 1 || backupCfg.DoMonthly > 31 {
		return fmt.Errorf("invalid day of month for monthly backups (must be 1-31)")
	}

	// Validate DB types
	if len(backupCfg.DBTypes) == 0 {
		return fmt.Errorf("no database types specified for backup")
	}

	// Test if we can access the backup directory
	testFile := filepath.Join(backupCfg.BackupDir, ".pruvon_backup_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("cannot write to backup directory: %v", err)
	}
	os.Remove(testFile)

	return nil
}

// CheckBackupPrerequisites checks if all prerequisites for backup are met
func CheckBackupPrerequisites(cfg *config.Config) error {
	// Check if dokku is installed
	if _, err := os.Stat("/usr/bin/dokku"); os.IsNotExist(err) {
		// Try finding dokku in PATH
		if _, err = exec.LookPath("dokku"); err != nil {
			return fmt.Errorf("dokku command not found in /usr/bin/dokku or PATH")
		}
	}

	// Check if gzip is installed
	if _, err := exec.LookPath("gzip"); err != nil {
		return fmt.Errorf("gzip command not found in PATH")
	}

	// Initialize and validate backup configuration
	if err := InitBackupConfig(cfg); err != nil {
		return err
	}

	return nil
}
