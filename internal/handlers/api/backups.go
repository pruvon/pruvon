package api

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/models"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Cache for backup results with an expiration time
var (
	backupListCache      = make(map[string]backupListCacheEntry)
	backupListCacheMutex sync.RWMutex
)

type backupListCacheEntry struct {
	Backups    []models.BackupFile
	Expiration time.Time
}

// Cache expiration time - 5 minutes
const backupCacheExpiration = 5 * time.Minute

// Cache for backup types with an expiration time
var (
	backupTypesCache      []string
	backupTypesCacheTime  time.Time
	backupTypesCacheMutex sync.RWMutex
)

// Cache expiration time for backup types - 30 minutes
const backupTypesCacheExpiration = 30 * time.Minute

func SetupBackupsRoutes(app *fiber.App) {
	app.Get("/api/backups/types", handleBackupTypes)
	app.Get("/api/backups/settings", handleGetBackupSettings)     // GET settings
	app.Post("/api/backups/settings", handleUpdateBackupSettings) // POST settings
	app.Get("/api/backups/:dbType/:backupType", handleBackupList)
	app.Get("/api/backups/download/:dbType/:backupType/:database/:file", handleBackupDownload)
}

// handleGetBackupSettings retrieves the current backup configuration
func handleGetBackupSettings(c *fiber.Ctx) error {
	cfg := config.GetConfig()
	backupCfg := cfg.Backup

	if backupCfg == nil {
		// Return default settings if none exist in config file
		backupCfg = &config.BackupConfig{
			BackupDir:      config.DefaultBackupDir, // Use constant for default
			DoWeekly:       0,                       // Sunday (0=Sunday in Go's time.Weekday)
			DoMonthly:      1,
			DBTypes:        config.DefaultBackupDBTypes, // Use constant for default
			KeepDailyDays:  7,
			KeepWeeklyNum:  6,
			KeepMonthlyNum: 3,
		}
	} else {
		// Ensure DBTypes is never nil, return empty slice if it is
		if backupCfg.DBTypes == nil {
			backupCfg.DBTypes = []string{}
		}
		// Ensure default values if specific fields are missing or zero/empty
		if backupCfg.BackupDir == "" {
			backupCfg.BackupDir = config.DefaultBackupDir
		}
		if backupCfg.KeepDailyDays <= 0 {
			backupCfg.KeepDailyDays = 7
		}
		if backupCfg.KeepWeeklyNum <= 0 {
			backupCfg.KeepWeeklyNum = 6
		}
		if backupCfg.KeepMonthlyNum <= 0 {
			backupCfg.KeepMonthlyNum = 3
		}
	}

	return c.JSON(backupCfg)
}

// handleUpdateBackupSettings updates the backup configuration
func handleUpdateBackupSettings(c *fiber.Ctx) error {
	newBackupSettings := new(config.BackupConfig)

	if err := c.BodyParser(newBackupSettings); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Ensure DBTypes is not nil, default to empty slice if it is
	if newBackupSettings.DBTypes == nil {
		newBackupSettings.DBTypes = []string{}
	}

	// Basic validation (can be expanded)
	// Validate do_monthly and do_weekly ranges
	if newBackupSettings.DoMonthly < 1 || newBackupSettings.DoMonthly > 31 {
		newBackupSettings.DoMonthly = 1
	}
	if newBackupSettings.DoWeekly < 0 || newBackupSettings.DoWeekly > 7 {
		newBackupSettings.DoWeekly = 0
	}
	if newBackupSettings.KeepDailyDays <= 0 {
		newBackupSettings.KeepDailyDays = 7 // Default if invalid
	}
	if newBackupSettings.KeepWeeklyNum <= 0 {
		newBackupSettings.KeepWeeklyNum = 6
	}
	if newBackupSettings.KeepMonthlyNum <= 0 {
		newBackupSettings.KeepMonthlyNum = 3
	}
	if newBackupSettings.BackupDir == "" {
		newBackupSettings.BackupDir = config.DefaultBackupDir // Use constant
	}

	// Get current full config
	cfg := config.GetConfig()
	if cfg == nil {
		// This case should ideally not happen if config is loaded at startup
		// but handle it defensively
		cfg = &config.Config{}
	}

	// Update the backup part
	cfg.Backup = newBackupSettings

	// Save the updated config
	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err) // Log the error server-side
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save configuration",
		})
	}

	// Return the saved settings, ensuring DBTypes is not nil
	if cfg.Backup.DBTypes == nil {
		cfg.Backup.DBTypes = []string{}
	}
	return c.JSON(cfg.Backup)
}

// getBackupDir returns the backup directory from config or the default
func getBackupDir() string {
	cfg := config.GetConfig()
	if cfg != nil && cfg.Backup != nil && cfg.Backup.BackupDir != "" {
		return cfg.Backup.BackupDir
	}
	return "/var/lib/dokku/data/pruvon-backup" // Default backup directory
}

func handleBackupTypes(c *fiber.Ctx) error {
	// Check cache first
	backupTypesCacheMutex.RLock()
	if !backupTypesCacheTime.IsZero() && time.Since(backupTypesCacheTime) < backupTypesCacheExpiration {
		types := backupTypesCache
		backupTypesCacheMutex.RUnlock()
		return c.JSON(fiber.Map{"types": types})
	}
	backupTypesCacheMutex.RUnlock()

	// Get types from filesystem
	backupDir := getBackupDir()
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return c.JSON(fiber.Map{"types": []string{}})
	}

	var types []string
	for _, f := range files {
		if f.IsDir() {
			types = append(types, f.Name())
		}
	}

	// Update cache
	backupTypesCacheMutex.Lock()
	backupTypesCache = types
	backupTypesCacheTime = time.Now()
	backupTypesCacheMutex.Unlock()

	return c.JSON(fiber.Map{"types": types})
}

func handleBackupList(c *fiber.Ctx) error {
	dbType := c.Params("dbType")
	backupType := c.Params("backupType")

	// Create a cache key
	cacheKey := fmt.Sprintf("%s:%s", dbType, backupType)

	// Check cache first
	backupListCacheMutex.RLock()
	if entry, found := backupListCache[cacheKey]; found && time.Now().Before(entry.Expiration) {
		backupListCacheMutex.RUnlock()
		return c.JSON(fiber.Map{"backups": entry.Backups})
	}
	backupListCacheMutex.RUnlock()

	backupDir := getBackupDir()
	basePath := filepath.Join(backupDir, dbType, backupType)

	// Check if path exists first
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return c.JSON(fiber.Map{"backups": []models.BackupFile{}})
	}

	var backups []models.BackupFile

	// Use filepath.Walk for a single pass through the directory structure
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}

		// Skip the root directory and database directories
		if path == basePath || filepath.Dir(path) == basePath {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get database name from parent directory
		parentDir := filepath.Base(filepath.Dir(path))

		backups = append(backups, models.BackupFile{
			Name:     info.Name(),
			Size:     info.Size(),
			Database: parentDir,
		})

		return nil
	})

	if err != nil {
		return c.JSON(fiber.Map{"backups": []models.BackupFile{}})
	}

	// Update cache
	backupListCacheMutex.Lock()
	backupListCache[cacheKey] = backupListCacheEntry{
		Backups:    backups,
		Expiration: time.Now().Add(backupCacheExpiration),
	}
	backupListCacheMutex.Unlock()

	return c.JSON(fiber.Map{"backups": backups})
}

func handleBackupDownload(c *fiber.Ctx) error {
	dbType := c.Params("dbType")
	backupType := c.Params("backupType")
	database := c.Params("database")
	fileName := c.Params("file")

	backupDir := getBackupDir()
	filePath := filepath.Join(backupDir, dbType, backupType, database, fileName)

	return c.Download(filePath)
}
