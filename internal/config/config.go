package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v2"
)

const (
	DefaultConfigPath = "pruvon.yml"
	DefaultBackupDir  = "/var/lib/dokku/data/pruvon-backup"
)

var (
	DefaultBackupDBTypes = []string{"postgres", "mariadb", "mongo", "redis"}

	// Global config state protected by a single mutex
	mu            sync.RWMutex
	currentConfig *Config
	configPath    string
)

// Config represents the main configuration structure loaded from config.yaml
// We use yaml tags for reading from the file and json tags for API responses
type Config struct {
	Admin struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
	GitHub struct {
		ClientID     string       `yaml:"client_id"`
		ClientSecret string       `yaml:"client_secret"`
		Users        []GitHubUser `yaml:"users"`
	} `yaml:"github"`
	Pruvon struct {
		Listen string `yaml:"listen"`
	} `yaml:"pruvon"`
	Dokku struct {
	} `yaml:"dokku"`
	Server *ServerConfig `yaml:"server" json:"server"`
	Backup *BackupConfig `yaml:"backup" json:"backup"`
}

// ServerConfig holds server-related settings
type ServerConfig struct {
	Port string `yaml:"port" json:"port"`
	Host string `yaml:"host" json:"host"`
}

// BackupConfig holds backup-related settings
type BackupConfig struct {
	BackupDir      string   `yaml:"backup_dir" json:"backup_dir"`
	DoWeekly       int      `yaml:"do_weekly" json:"do_weekly"`   // Day of week (0=Sunday, 1=Monday, ..., 6=Saturday)
	DoMonthly      int      `yaml:"do_monthly" json:"do_monthly"` // Day of month
	DBTypes        []string `yaml:"db_types" json:"db_types"`
	KeepDailyDays  int      `yaml:"keep_daily_days" json:"keep_daily_days"`
	KeepWeeklyNum  int      `yaml:"keep_weekly_num" json:"keep_weekly_num"`
	KeepMonthlyNum int      `yaml:"keep_monthly_num" json:"keep_monthly_num"`
}

type GitHubUser struct {
	Username string              `yaml:"username"`
	Routes   []string            `yaml:"routes"`
	Apps     []string            `yaml:"apps,omitempty"`
	Services map[string][]string `yaml:"services,omitempty"`
}

func init() {
	// Initialize with default path, can be overridden by LoadConfig
	dir, err := os.Getwd()
	if err != nil {
		// Fallback to current directory if Getwd fails
		configPath = DefaultConfigPath
		return
	}
	configPath = filepath.Join(dir, DefaultConfigPath)
}

// GetConfig returns a copy of the current config (thread-safe)
func GetConfig() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return currentConfig
}

// UpdateConfig updates the current config in memory (thread-safe)
func UpdateConfig(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	currentConfig = cfg
}

// GetConfigPath returns the path of the currently loaded configuration file (thread-safe)
func GetConfigPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return configPath
}

// LoadConfig loads configuration from a file and updates the global state (thread-safe)
func LoadConfig(path string) (*Config, error) {
	// Read file without holding lock
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Parse YAML
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Update global state with write lock
	mu.Lock()
	currentConfig = cfg
	configPath = path
	mu.Unlock()

	return cfg, nil
}

// SaveConfig saves the configuration to file and updates global state (thread-safe)
func SaveConfig(cfg *Config) error {
	// Get current path under read lock
	mu.RLock()
	path := configPath
	mu.RUnlock()

	// Marshal YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling config: %v", err)
	}

	// Write to temporary file first (atomic write pattern)
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, yamlData, 0644); err != nil {
		return fmt.Errorf("error writing temporary config file: %v", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("error replacing config file: %v", err)
	}

	// Update global state with write lock
	mu.Lock()
	currentConfig = cfg
	mu.Unlock()

	return nil
}
