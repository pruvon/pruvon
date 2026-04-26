package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

const (
	DefaultConfigPath = "pruvon.yml"
	DefaultBackupDir  = "/var/lib/dokku/data/pruvon-backup"
	configFileMode    = 0600
)

var (
	DefaultBackupDBTypes = []string{"postgres", "mariadb", "mongo", "redis"}

	// Global config state protected by a single mutex
	mu            sync.RWMutex
	currentConfig *Config
	configPath    string
)

// Config represents the canonical configuration structure used at runtime.
type Config struct {
	Users  []User `yaml:"users,omitempty" json:"users,omitempty"`
	Pruvon struct {
		Listen string `yaml:"listen"`
	} `yaml:"pruvon"`
	Dokku  struct{}      `yaml:"dokku"`
	Server *ServerConfig `yaml:"server" json:"server"`
	Backup *BackupConfig `yaml:"backup" json:"backup"`
}

// User is the canonical persisted and runtime user model.
type User struct {
	Username string              `yaml:"username" json:"username"`
	Password string              `yaml:"password,omitempty" json:"-"`
	Role     string              `yaml:"role" json:"role"`
	Routes   []string            `yaml:"routes,omitempty" json:"routes,omitempty"`
	Apps     []string            `yaml:"apps,omitempty" json:"apps,omitempty"`
	Services map[string][]string `yaml:"services,omitempty" json:"services,omitempty"`
	GitHub   *UserGitHub         `yaml:"github,omitempty" json:"github,omitempty"`
	Disabled bool                `yaml:"disabled,omitempty" json:"disabled"`
}

// UserGitHub stores optional GitHub metadata for SSH key sync.
type UserGitHub struct {
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
}

// LegacyGitHubUser remains only for config compatibility while loading old files.
type LegacyGitHubUser struct {
	Username string              `yaml:"username"`
	Routes   []string            `yaml:"routes"`
	Apps     []string            `yaml:"apps,omitempty"`
	Services map[string][]string `yaml:"services,omitempty"`
}

// ServerConfig holds server-related settings.
type ServerConfig struct {
	Port string `yaml:"port" json:"port"`
	Host string `yaml:"host" json:"host"`
}

// BackupConfig holds backup-related settings.
type BackupConfig struct {
	BackupDir      string   `yaml:"backup_dir" json:"backup_dir"`
	DoWeekly       int      `yaml:"do_weekly" json:"do_weekly"`
	DoMonthly      int      `yaml:"do_monthly" json:"do_monthly"`
	DBTypes        []string `yaml:"db_types" json:"db_types"`
	KeepDailyDays  int      `yaml:"keep_daily_days" json:"keep_daily_days"`
	KeepWeeklyNum  int      `yaml:"keep_weekly_num" json:"keep_weekly_num"`
	KeepMonthlyNum int      `yaml:"keep_monthly_num" json:"keep_monthly_num"`
}

type rawConfig struct {
	Users []User `yaml:"users,omitempty"`
	Admin *struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin,omitempty"`
	GitHub *struct {
		ClientID     string             `yaml:"client_id"`
		ClientSecret string             `yaml:"client_secret"`
		Users        []LegacyGitHubUser `yaml:"users"`
	} `yaml:"github,omitempty"`
	Pruvon struct {
		Listen string `yaml:"listen"`
	} `yaml:"pruvon"`
	Dokku  struct{}      `yaml:"dokku"`
	Server *ServerConfig `yaml:"server"`
	Backup *BackupConfig `yaml:"backup"`
}

func init() {
	// Initialize with default path, can be overridden by LoadConfig
	dir, err := os.Getwd()
	if err != nil {
		configPath = DefaultConfigPath
		return
	}
	configPath = filepath.Join(dir, DefaultConfigPath)
}

// GetConfig returns the current config pointer protected by a read lock.
func GetConfig() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return currentConfig
}

// UpdateConfig updates the current config in memory.
func UpdateConfig(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	currentConfig = cfg
}

// GetConfigPath returns the path of the currently loaded config file.
func GetConfigPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return configPath
}

// LoadConfig loads configuration from a file, normalizes legacy auth structures,
// and updates the global runtime state.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	cfg, err := normalizeRawConfig(&raw)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	currentConfig = cfg
	configPath = path
	mu.Unlock()

	return cfg, nil
}

// SaveConfig validates and saves the canonical configuration back to disk.
func SaveConfig(cfg *Config) error {
	if err := ValidateUsers(cfg.Users); err != nil {
		return err
	}

	mu.RLock()
	path := configPath
	mu.RUnlock()

	if err := WriteConfigFile(path, cfg); err != nil {
		return err
	}

	mu.Lock()
	if currentConfig == nil || currentConfig == cfg {
		currentConfig = cfg
	} else {
		*currentConfig = *cfg
	}
	mu.Unlock()

	return nil
}

// WriteConfigFile marshals and atomically writes a canonical config file.
func WriteConfigFile(path string, cfg *Config) error {
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling config: %v", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, yamlData, configFileMode); err != nil {
		if canOverwriteConfigInPlace(path, err) {
			if overwriteErr := overwriteConfigFile(path, yamlData); overwriteErr == nil {
				return nil
			} else {
				return fmt.Errorf("error writing temporary config file: %v (fallback overwrite failed: %v)", err, overwriteErr)
			}
		}
		return fmt.Errorf("error writing temporary config file: %v", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		if canOverwriteConfigInPlace(path, err) {
			if overwriteErr := overwriteConfigFile(path, yamlData); overwriteErr == nil {
				return nil
			} else {
				return fmt.Errorf("error replacing config file: %v (fallback overwrite failed: %v)", err, overwriteErr)
			}
		}
		return fmt.Errorf("error replacing config file: %v", err)
	}

	return nil
}

func canOverwriteConfigInPlace(path string, originalErr error) bool {
	if !os.IsPermission(originalErr) {
		return false
	}

	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func overwriteConfigFile(path string, yamlData []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := file.Chmod(configFileMode); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := file.Write(yamlData); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}

	return nil
}

// Normalize ensures the config uses canonical runtime defaults.
func (c *Config) Normalize() {
	for i := range c.Users {
		c.Users[i].Normalize()
	}
	if c.Backup == nil {
		return
	}
	if len(c.Backup.DBTypes) == 0 {
		c.Backup.DBTypes = append([]string(nil), DefaultBackupDBTypes...)
	}
}

// Normalize ensures a user uses the canonical representation.
func (u *User) Normalize() {
	u.Role = normalizeRole(u.Role)
	if len(u.Routes) == 0 {
		u.Routes = nil
	}
	if len(u.Apps) == 0 {
		u.Apps = nil
	}
	if len(u.Services) == 0 {
		u.Services = nil
	}
	if u.GitHub != nil && strings.TrimSpace(u.GitHub.Username) == "" {
		u.GitHub = nil
	}
	if u.GitHub != nil {
		u.GitHub.Username = strings.TrimSpace(u.GitHub.Username)
	}
	u.Username = strings.TrimSpace(u.Username)
	if u.Services != nil {
		for svcType, names := range u.Services {
			if len(names) == 0 {
				delete(u.Services, svcType)
			}
		}
		if len(u.Services) == 0 {
			u.Services = nil
		}
	}
}

// FindUser returns the canonical user with the matching username.
func (c *Config) FindUser(username string) *User {
	if c == nil {
		return nil
	}
	for i := range c.Users {
		if c.Users[i].Username == username {
			return &c.Users[i]
		}
	}
	return nil
}

// FindEnabledAdminCount returns the number of enabled admin users.
func (c *Config) FindEnabledAdminCount() int {
	if c == nil {
		return 0
	}
	count := 0
	for _, user := range c.Users {
		if user.Role == RoleAdmin && !user.Disabled {
			count++
		}
	}
	return count
}

// FindUserByUsername is a convenience helper around the global config state.
func FindUserByUsername(username string) *User {
	cfg := GetConfig()
	if cfg == nil {
		return nil
	}
	return cfg.FindUser(username)
}

// ValidateUsers validates canonical user definitions before save or use.
func ValidateUsers(users []User) error {
	seen := make(map[string]struct{}, len(users))
	for _, user := range users {
		normalized := user
		normalized.Normalize()
		if normalized.Username == "" {
			return errors.New("user username cannot be empty")
		}
		if normalized.Role != RoleAdmin && normalized.Role != RoleUser {
			return fmt.Errorf("user %q has invalid role %q", normalized.Username, normalized.Role)
		}
		if _, exists := seen[normalized.Username]; exists {
			return fmt.Errorf("duplicate username %q in config", normalized.Username)
		}
		seen[normalized.Username] = struct{}{}
	}
	return nil
}

func normalizeRawConfig(raw *rawConfig) (*Config, error) {
	cfg := &Config{
		Users:  append([]User(nil), raw.Users...),
		Server: raw.Server,
		Backup: raw.Backup,
	}
	cfg.Pruvon = raw.Pruvon
	cfg.Dokku = raw.Dokku

	if len(cfg.Users) == 0 {
		legacyUsers, err := normalizeLegacyUsers(raw)
		if err != nil {
			return nil, err
		}
		cfg.Users = legacyUsers
	}

	cfg.Normalize()
	if err := ValidateUsers(cfg.Users); err != nil {
		return nil, fmt.Errorf("invalid config users: %w", err)
	}

	return cfg, nil
}

func normalizeLegacyUsers(raw *rawConfig) ([]User, error) {
	users := make([]User, 0)
	if raw.Admin != nil && (raw.Admin.Username != "" || raw.Admin.Password != "") {
		users = append(users, User{
			Username: raw.Admin.Username,
			Password: raw.Admin.Password,
			Role:     RoleAdmin,
		})
	}

	if raw.GitHub != nil {
		for _, legacy := range raw.GitHub.Users {
			users = append(users, User{
				Username: legacy.Username,
				Role:     RoleUser,
				Routes:   append([]string(nil), legacy.Routes...),
				Apps:     append([]string(nil), legacy.Apps...),
				Services: cloneServices(legacy.Services),
				GitHub: &UserGitHub{
					Username: legacy.Username,
				},
			})
		}
	}

	if err := ValidateUsers(users); err != nil {
		return nil, err
	}

	if raw.GitHub != nil && len(raw.GitHub.Users) > 0 && !hasLegacyLocalAdminUser(users) {
		return nil, errors.New("legacy github.users configs require a local admin with a password before migration; add an admin user and retry")
	}

	return users, nil
}

func hasLegacyLocalAdminUser(users []User) bool {
	for _, user := range users {
		if user.Role == RoleAdmin && strings.TrimSpace(user.Username) != "" && strings.TrimSpace(user.Password) != "" && !user.Disabled {
			return true
		}
	}

	return false
}

func cloneServices(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for svcType, names := range in {
		out[svcType] = append([]string(nil), names...)
	}
	return out
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

func normalizeRole(role string) string {
	if role == "" {
		return RoleUser
	}
	return role
}
