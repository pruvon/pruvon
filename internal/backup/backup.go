package backup

import (
	"bytes"
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BackupConfig holds the configuration for backup operations
type BackupConfig struct {
	BackupDir      string   `yaml:"backup_dir"`
	DoWeekly       int      `yaml:"do_weekly"`  // Day of week (0-6 where 0 is Sunday)
	DoMonthly      int      `yaml:"do_monthly"` // Day of month (1-31)
	DBTypes        []string `yaml:"db_types"`   // Types of databases to backup (postgres, mariadb, etc.)
	KeepWeeklyNum  int      `yaml:"keep_weekly_num"`
	KeepDailyDays  int      `yaml:"keep_daily_days"`
	KeepMonthlyNum int      `yaml:"keep_monthly_num"`
}

// TimeData contains time-related information for the backup
type TimeData struct {
	DateTime        string
	DayOfWeek       string
	DayNumberOfWeek int
	DayOfMonth      int
	Month           string
	WeekNumber      int
}

func dokkuCommand(args ...string) *exec.Cmd {
	sudoArgs := append([]string{"-n", "-u", "dokku", "dokku"}, args...)
	return exec.Command("sudo", sudoArgs...)
}

// GetTimeData returns current time information needed for backups
func GetTimeData() TimeData {
	now := time.Now()

	// Convert from Go's time.Weekday (0=Sunday) to ISO weekday (1=Monday, 7=Sunday)
	isoWeekday := int(now.Weekday())
	if isoWeekday == 0 {
		isoWeekday = 7 // Convert Sunday from 0 to 7
	}

	return TimeData{
		DateTime:        now.Format("2006-01-02_15h04m"),
		DayOfWeek:       now.Weekday().String(),
		DayNumberOfWeek: isoWeekday, // Using ISO weekday (1=Monday, 7=Sunday)
		DayOfMonth:      now.Day(),
		Month:           now.Month().String(),
		WeekNumber:      getWeekNumber(now),
	}
}

// getWeekNumber returns the ISO 8601 week number
func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

// Backup performs backup operations based on the configuration
func Backup(backupType string, cfg *config.Config) error {
	// Check prerequisites and initialize backup configuration
	if err := CheckBackupPrerequisites(cfg); err != nil {
		return fmt.Errorf("backup prerequisites check failed: %v", err)
	}

	// Create backup directory structure
	if err := ensureBackupDirs(cfg.Backup.BackupDir, cfg.Backup.DBTypes); err != nil {
		return err
	}

	timeData := GetTimeData()

	// Determine which backup type to run if not specified
	if backupType == "auto" {
		if timeData.DayOfMonth == cfg.Backup.DoMonthly {
			backupType = "monthly"
		} else if (timeData.DayNumberOfWeek == cfg.Backup.DoWeekly) ||
			// Special case: If configured for Sunday (0 or 7) and today is Sunday
			((cfg.Backup.DoWeekly == 0 || cfg.Backup.DoWeekly == 7) && timeData.DayNumberOfWeek == 7) {
			backupType = "weekly"
		} else {
			backupType = "daily"
		}
	}

	// Run backup for each database type specified in config
	for _, dbType := range cfg.Backup.DBTypes {
		// Normalize dbType for plugin check (mongodb -> mongo)
		pluginName := dbType
		if pluginName == "mongodb" {
			pluginName = "mongo"
		}

		// Check if the required plugin is installed
		installed, err := isPluginInstalled(pluginName)
		if err != nil {
			// Error checking plugin, log and continue
			fmt.Printf("Warning: Could not check if plugin '%s' is installed: %v\n", pluginName, err)
			continue
		}

		if !installed {
			fmt.Printf("Notice: Skipping backup for %s as the '%s' plugin is not installed.\n", dbType, pluginName)
			continue
		}

		// Plugin is installed, proceed with backup
		fmt.Printf("Backing up databases of type: %s\n", dbType)
		if err := backupDatabases(dbType, backupType, cfg.Backup.BackupDir, timeData, cfg.Backup.KeepWeeklyNum); err != nil {
			fmt.Printf("Warning: Backup failed for database type %s: %v\n", dbType, err)
			// Optionally decide if one failure should stop the whole process or just log and continue
			// For now, we log and continue to backup other types
		}
	}

	// Set proper permissions on the backup directory
	if err := os.Chmod(cfg.Backup.BackupDir, 0750); err != nil {
		return fmt.Errorf("failed to set permissions on backup directory: %v", err)
	}

	// Clean up old backups
	if err := CleanupOldBackups(cfg.Backup.BackupDir, cfg.Backup.DBTypes, cfg); err != nil {
		fmt.Printf("Warning: Failed to clean up old backups: %v\n", err)
	}

	return nil
}

// ensureBackupDirs creates the necessary directory structure for backups
func ensureBackupDirs(backupDir string, dbTypes []string) error {
	for _, dbType := range dbTypes {
		for _, rotateType := range []string{"daily", "weekly", "monthly"} {
			dirPath := filepath.Join(backupDir, dbType, rotateType)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create backup directory %s: %v", dirPath, err)
			}
		}
	}
	return nil
}

// backupDatabases performs backup for a specific database type
func backupDatabases(dbType, rotateType, backupDir string, timeData TimeData, keepWeeklyNum int) error {
	// Get list of databases from dokku
	databases, err := listDatabases(dbType)
	if err != nil {
		return err
	}

	baseDir := filepath.Join(backupDir, dbType, rotateType)

	for _, db := range databases {
		dbDir := filepath.Join(baseDir, db)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("failed to create database backup directory %s: %v", dbDir, err)
		}

		// Perform the backup based on type
		if err := performBackup(dbType, db, rotateType, dbDir, timeData, keepWeeklyNum); err != nil {
			return err
		}
	}

	return nil
}

// performBackup performs the actual database backup
func performBackup(dbType, db, rotateType, dbDir string, timeData TimeData, keepWeeklyNum int) error {
	fileExt := getFileExtension(dbType)

	switch rotateType {
	case "monthly":
		filename := fmt.Sprintf("%s_%s.%s.%s.%s", db, rotateType, timeData.Month, timeData.DateTime, fileExt)
		outputPath := filepath.Join(dbDir, filename)
		if err := exportDatabase(dbType, db, outputPath); err != nil {
			return err
		}
		return compressFile(outputPath)

	case "weekly":
		// Remove old weekly backup
		remWeek := calculateRemWeek(timeData.WeekNumber, keepWeeklyNum)
		oldPattern := filepath.Join(dbDir, fmt.Sprintf("%s_%s.%s.*", db, rotateType, remWeek))
		_ = removeFiles(oldPattern)

		// Create new backup
		filename := fmt.Sprintf("%s_%s.%d.%s.%s", db, rotateType, timeData.WeekNumber, timeData.DateTime, fileExt)
		outputPath := filepath.Join(dbDir, filename)
		if err := exportDatabase(dbType, db, outputPath); err != nil {
			return err
		}
		return compressFile(outputPath)

	case "daily":
		// Remove old daily backup for this day of week
		oldPattern := filepath.Join(dbDir, fmt.Sprintf("*.%s.%s", timeData.DayOfWeek, fileExt))
		_ = removeFiles(oldPattern)

		// Create new backup
		filename := fmt.Sprintf("%s_%s.%s.%s", db, timeData.DateTime, timeData.DayOfWeek, fileExt)
		outputPath := filepath.Join(dbDir, filename)
		if err := exportDatabase(dbType, db, outputPath); err != nil {
			return err
		}
		return compressFile(outputPath)
	}

	return nil
}

// calculateRemWeek calculates the week number to remove for weekly rotation
func calculateRemWeek(weekNumber int, keepWeeklyNum int) string {
	var remWeek int

	if weekNumber <= keepWeeklyNum {
		remWeek = 53 - keepWeeklyNum + weekNumber
	} else if weekNumber < 10+keepWeeklyNum {
		remWeek = weekNumber - keepWeeklyNum
		return fmt.Sprintf("0%d", remWeek) // Add leading zero
	} else {
		remWeek = weekNumber - keepWeeklyNum
	}

	return strconv.Itoa(remWeek)
}

// getFileExtension returns the appropriate file extension based on the database type
func getFileExtension(dbType string) string {
	switch dbType {
	case "mariadb", "mysql":
		return "sql"
	case "redis":
		return "rdb"
	case "mongo", "mongodb":
		return "archive"
	default:
		return "dump" // default for postgres
	}
}

// listDatabases gets the list of databases from dokku
func listDatabases(dbType string) ([]string, error) {
	// Normalize mongodb to mongo for dokku command
	dokkuDbType := dbType
	if dbType == "mongodb" {
		dokkuDbType = "mongo"
	}

	cmd := dokkuCommand(dokkuDbType + ":list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list %s databases: %v", dbType, err)
	}

	lines := strings.Split(string(output), "\n")
	var databases []string

	// Skip the header line and parse database names
	for i, line := range lines {
		if i > 0 && len(line) > 0 {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				databases = append(databases, fields[0])
			}
		}
	}

	return databases, nil
}

// isPluginInstalled checks if a specific dokku plugin is installed
func isPluginInstalled(pluginName string) (bool, error) {
	cmd := dokkuCommand("plugin:list")
	output, err := cmd.Output()
	if err != nil {
		// If dokku or plugin:list command fails, assume plugin is not installed
		// Log the error for debugging but don't fail the backup entirely
		fmt.Printf("Warning: Failed to list dokku plugins: %v\n", err)
		return false, nil
	}

	pluginList := string(output)
	// Check if the plugin name exists as a whole word/line in the list
	pattern := fmt.Sprintf(`(?m)^%s$`, pluginName) // Match whole line
	match, _ := regexp.MatchString(pattern, pluginList)
	if !match {
		// Also check if it's part of a line like "00_dokku-standard   mongo    enabled   0.30.0  dokku mongo plugin"
		pattern = fmt.Sprintf(`\s%s\s`, pluginName) // Match surrounded by spaces
		match, _ = regexp.MatchString(pattern, pluginList)
	}

	return match, nil
}

// exportDatabase exports the database using dokku
func exportDatabase(dbType, db, filename string) error {
	// Normalize dbType for dokku commands if needed (e.g., mongodb -> mongo)
	dokkuDbType := dbType
	if dbType == "mongodb" {
		dokkuDbType = "mongo"
	}

	// Run dokku export command directly without script wrapper
	cmd := dokkuCommand(dokkuDbType+":export", db)

	// Set environment variables to prevent tty errors and ensure clean output
	cmd.Env = append(os.Environ(),
		"TERM=dumb",
		"DOKKU_QUIET_OUTPUT=1",
	)

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to export database %s (%s): %v\nError output: %s", db, dbType, err, stderr.String())
	}

	// Write the output to the file
	if err := os.WriteFile(filename, stdout.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write backup file %s: %v", filename, err)
	}

	return nil
}

// compressFile compresses the backup file using gzip
func compressFile(filename string) error {
	cmd := exec.Command("gzip", "-f", filename)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to compress file %s: %v", filename, err)
	}
	return nil
}

// removeFiles removes files matching the given pattern
func removeFiles(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			fmt.Printf("Warning: Failed to remove file %s: %v\n", match, err)
		}
	}

	return nil
}

// CleanupOldBackups removes old backups based on configuration
func CleanupOldBackups(backupDir string, dbTypes []string, cfg *config.Config) error {
	// Clean up daily backups (older than KeepDailyDays)
	if err := cleanupDailyBackups(backupDir, dbTypes, cfg.Backup.KeepDailyDays); err != nil {
		return err
	}

	// Clean up monthly backups (keep only the last KeepMonthlyNum months)
	if err := cleanupMonthlyBackups(backupDir, dbTypes, cfg.Backup.KeepMonthlyNum); err != nil {
		return err
	}

	return nil
}

// cleanupDailyBackups removes daily backups older than specified days
func cleanupDailyBackups(backupDir string, dbTypes []string, keepDays int) error {
	// Calculate the cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -keepDays)
	fmt.Printf("Removing daily backups older than %s\n", cutoffDate.Format("2006-01-02"))

	for _, dbType := range dbTypes {
		dailyBackupDir := filepath.Join(backupDir, dbType, "daily")

		// Check if directory exists
		if _, err := os.Stat(dailyBackupDir); os.IsNotExist(err) {
			continue
		}

		// Read database directories
		dbDirs, err := os.ReadDir(dailyBackupDir)
		if err != nil {
			return fmt.Errorf("failed to read daily backup directory for %s: %v", dbType, err)
		}

		// Process each database directory
		for _, dbDir := range dbDirs {
			if !dbDir.IsDir() {
				continue
			}

			dbPath := filepath.Join(dailyBackupDir, dbDir.Name())
			files, err := os.ReadDir(dbPath)
			if err != nil {
				fmt.Printf("Warning: Failed to read backup files in %s: %v\n", dbPath, err)
				continue
			}

			// Process each backup file
			for _, file := range files {
				if file.IsDir() {
					continue
				}

				fileName := file.Name()
				filePath := filepath.Join(dbPath, fileName)

				// Extract date from filename (format: database_name_YYYY-MM-DD_HHhMMm.DayOfWeek.ext.gz)
				// We're looking for the date part which is after the first underscore
				parts := strings.Split(fileName, "_")
				if len(parts) < 2 {
					continue
				}

				// Try to find a date part in the format YYYY-MM-DD
				var fileDate time.Time
				var parseErr error
				for _, part := range parts {
					if len(part) >= 10 && strings.Count(part[:10], "-") == 2 {
						// This looks like a date part
						fileDate, parseErr = time.Parse("2006-01-02", part[:10])
						if parseErr == nil {
							break
						}
					}
				}

				if parseErr != nil {
					// Couldn't parse a date from this filename
					continue
				}

				// If the file is older than cutoff date, remove it
				if fileDate.Before(cutoffDate) {
					fmt.Printf("Removing old backup: %s\n", filePath)
					if err := os.Remove(filePath); err != nil {
						fmt.Printf("Warning: Failed to remove old backup %s: %v\n", filePath, err)
					}
				}
			}
		}
	}

	return nil
}

// cleanupMonthlyBackups removes monthly backups keeping only the specified number
func cleanupMonthlyBackups(backupDir string, dbTypes []string, keepMonths int) error {
	// Calculate the cutoff date
	cutoffDate := time.Now().AddDate(0, -keepMonths, 0)
	fmt.Printf("Removing monthly backups older than %s\n", cutoffDate.Format("2006-01-02"))

	// Get today's date to prevent deleting today's backups
	today := time.Now().Format("2006-01-02")

	for _, dbType := range dbTypes {
		monthlyBackupDir := filepath.Join(backupDir, dbType, "monthly")

		// Check if directory exists
		if _, err := os.Stat(monthlyBackupDir); os.IsNotExist(err) {
			continue
		}

		// Read database directories
		dbDirs, err := os.ReadDir(monthlyBackupDir)
		if err != nil {
			return fmt.Errorf("failed to read monthly backup directory for %s: %v", dbType, err)
		}

		// Process each database directory
		for _, dbDir := range dbDirs {
			if !dbDir.IsDir() {
				continue
			}

			dbPath := filepath.Join(monthlyBackupDir, dbDir.Name())
			files, err := os.ReadDir(dbPath)
			if err != nil {
				fmt.Printf("Warning: Failed to read backup files in %s: %v\n", dbPath, err)
				continue
			}

			// Group files by month to keep track of which ones to remove
			type MonthlyBackup struct {
				Path  string
				Date  time.Time
				Month string // Format: YYYY-MM
			}

			var backups []MonthlyBackup

			// Process each backup file
			for _, file := range files {
				if file.IsDir() {
					continue
				}

				fileName := file.Name()
				filePath := filepath.Join(dbPath, fileName)

				// Extract date from filename
				// Format is typically: db_name_monthly.Month.YYYY-MM-DD_HHhMMm.ext.gz
				parts := strings.Split(fileName, "_")
				if len(parts) < 2 {
					continue
				}

				// Try to find a date part in the format YYYY-MM-DD
				var fileDate time.Time
				var parseErr error
				for _, part := range parts {
					if len(part) >= 10 && strings.Count(part[:10], "-") == 2 {
						// This looks like a date part
						fileDate, parseErr = time.Parse("2006-01-02", part[:10])
						if parseErr == nil {
							break
						}
					}
				}

				if parseErr != nil {
					// Couldn't parse a date from this filename
					continue
				}

				// Group by year-month
				monthKey := fileDate.Format("2006-01")
				backups = append(backups, MonthlyBackup{
					Path:  filePath,
					Date:  fileDate,
					Month: monthKey,
				})
			}

			// Sort backups by date in descending order (newest first)
			sort.Slice(backups, func(i, j int) bool {
				return backups[i].Date.After(backups[j].Date)
			})

			// Keep track of months we've seen to keep only the most recent backup per month
			seenMonths := make(map[string]bool)

			// First pass: Keep one backup per month for the last KeepMonthlyNum months
			for _, backup := range backups {
				// Skip deleting today's backups
				if backup.Date.Format("2006-01-02") == today {
					seenMonths[backup.Month] = true
					continue // Always keep today's backup
				}

				// If this month is one of the last KeepMonthlyNum months, keep the first (most recent) backup
				if !backup.Date.Before(cutoffDate) {
					if !seenMonths[backup.Month] {
						seenMonths[backup.Month] = true
						continue // Keep this file
					}
				}

				// Otherwise, remove this backup
				fmt.Printf("Removing old monthly backup: %s\n", backup.Path)
				if err := os.Remove(backup.Path); err != nil {
					fmt.Printf("Warning: Failed to remove old backup %s: %v\n", backup.Path, err)
				}
			}
		}
	}

	return nil
}
