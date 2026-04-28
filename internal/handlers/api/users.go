package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	internallog "github.com/pruvon/pruvon/internal/log"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	servicelogs "github.com/pruvon/pruvon/internal/services/logs"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const strongPasswordError = "Password must be at least 8 characters and include an uppercase letter, lowercase letter, number, and symbol"

func SetupUsersRoutes(app *fiber.App) {
	app.Get("/api/settings/users", handleGetUsers)
	app.Get("/api/settings/user-options", handleGetUserOptions)
	app.Post("/api/settings/users", handleCreateUser)
	app.Put("/api/settings/users/:username", handleUpdateUser)
	app.Put("/api/settings/users/:username/password", handleUpdateUserPassword)
	app.Delete("/api/settings/users/:username", handleDeleteUser)
}

type userPayload struct {
	Username string              `json:"username"`
	Role     string              `json:"role"`
	Password string              `json:"password,omitempty"`
	Routes   []string            `json:"routes"`
	Apps     []string            `json:"apps"`
	Services map[string][]string `json:"services"`
	GitHub   struct {
		Username string `json:"username"`
	} `json:"github"`
	Disabled          bool `json:"disabled"`
	CanCreateApps     bool `json:"can_create_apps"`
	CanCreateServices bool `json:"can_create_services"`
}

func handleGetUsers(c *fiber.Ctx) error {
	if !requireAdminSettingsAccess(c) {
		return nil
	}

	cfg := config.GetConfig()
	currentUser := sessionUsername(c)

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       currentUser,
		Action:     "user_management_accessed",
		Method:     c.Method(),
		Route:      c.Path(),
		StatusCode: 200,
	})

	responseUsers := make([]fiber.Map, 0, len(cfg.Users))
	for _, user := range cfg.Users {
		githubUsername := ""
		if user.GitHub != nil {
			githubUsername = user.GitHub.Username
		}
		responseUsers = append(responseUsers, fiber.Map{
			"username":            user.Username,
			"role":                user.Role,
			"routes":              sliceOrEmpty(user.Routes),
			"apps":                sliceOrEmpty(user.Apps),
			"services":            servicesOrEmpty(user.Services),
			"github":              fiber.Map{"username": githubUsername},
			"disabled":            user.Disabled,
			"has_password":        user.Password != "",
			"can_create_apps":     user.CanCreateApps,
			"can_create_services": user.CanCreateServices,
		})
	}

	sort.Slice(responseUsers, func(i, j int) bool {
		return responseUsers[i]["username"].(string) < responseUsers[j]["username"].(string)
	})

	return c.JSON(fiber.Map{"users": responseUsers})
}

func handleCreateUser(c *fiber.Ctx) error {
	if !requireAdminSettingsAccess(c) {
		return nil
	}

	var req userPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	if strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password is required"})
	}

	cfg := config.GetConfig()
	if cfg.FindUser(strings.TrimSpace(req.Username)) != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "User already exists"})
	}

	user, err := buildUserFromPayload(req, true)
	if err != nil {
		return userRequestError(c, err)
	}

	updatedCfg := cloneConfig(cfg)
	updatedCfg.Users = append(updatedCfg.Users, *user)
	if err := config.SaveConfig(updatedCfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save config"})
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       sessionUsername(c),
		Action:     "user_added",
		Method:     c.Method(),
		Route:      c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{"username":%q}`, user.Username)),
		StatusCode: 200,
	})

	return c.SendStatus(fiber.StatusOK)
}

func handleUpdateUser(c *fiber.Ctx) error {
	if !requireAdminSettingsAccess(c) {
		return nil
	}

	targetUsername := c.Params("username")
	var req userPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	cfg := config.GetConfig()
	user := cfg.FindUser(targetUsername)
	if user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	updatedUser, err := buildUserFromPayload(req, false)
	if err != nil {
		return userRequestError(c, err)
	}
	updatedUser.Password = user.Password

	if updatedUser.Username != targetUsername && cfg.FindUser(updatedUser.Username) != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "User already exists"})
	}
	if err := validateUserMutation(cfg, targetUsername, updatedUser, sessionUsername(c)); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	updatedCfg := cloneConfig(cfg)
	for i := range updatedCfg.Users {
		if updatedCfg.Users[i].Username == targetUsername {
			updatedCfg.Users[i] = *updatedUser
			break
		}
	}

	if err := config.SaveConfig(updatedCfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save config"})
	}

	if targetUsername == sessionUsername(c) {
		if err := updateCurrentSessionUsername(c, updatedUser.Username, updatedUser.Role); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update session"})
		}
	}

	changes := fiber.Map{}
	if user.Username != updatedUser.Username {
		changes["old_username"] = user.Username
		changes["new_username"] = updatedUser.Username
	}
	if user.Role != updatedUser.Role {
		changes["old_role"] = user.Role
		changes["new_role"] = updatedUser.Role
	}
	oldGitHub := ""
	if user.GitHub != nil {
		oldGitHub = user.GitHub.Username
	}
	newGitHub := ""
	if updatedUser.GitHub != nil {
		newGitHub = updatedUser.GitHub.Username
	}
	if oldGitHub != newGitHub {
		changes["old_github_username"] = oldGitHub
		changes["new_github_username"] = newGitHub
	}
	if user.Disabled != updatedUser.Disabled {
		changes["old_disabled"] = user.Disabled
		changes["new_disabled"] = updatedUser.Disabled
	}
	if user.CanCreateApps != updatedUser.CanCreateApps {
		changes["old_can_create_apps"] = user.CanCreateApps
		changes["new_can_create_apps"] = updatedUser.CanCreateApps
	}
	if user.CanCreateServices != updatedUser.CanCreateServices {
		changes["old_can_create_services"] = user.CanCreateServices
		changes["new_can_create_services"] = updatedUser.CanCreateServices
	}

	params, _ := json.Marshal(changes)
	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       sessionUsername(c),
		Action:     "user_updated",
		Method:     c.Method(),
		Route:      c.Path(),
		Parameters: params,
		StatusCode: 200,
	})

	return c.SendStatus(fiber.StatusOK)
}

func handleUpdateUserPassword(c *fiber.Ctx) error {
	if !requireAdminSettingsAccess(c) {
		return nil
	}

	targetUsername := c.Params("username")
	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password is required"})
	}

	cfg := config.GetConfig()
	user := cfg.FindUser(targetUsername)
	if user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	updatedCfg := cloneConfig(cfg)
	for i := range updatedCfg.Users {
		if updatedCfg.Users[i].Username == targetUsername {
			updatedCfg.Users[i].Password = string(hashedPassword)
			break
		}
	}

	if err := config.SaveConfig(updatedCfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save config"})
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       sessionUsername(c),
		Action:     "user_password_updated",
		Method:     c.Method(),
		Route:      c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{"username":%q}`, targetUsername)),
		StatusCode: 200,
	})

	return c.SendStatus(fiber.StatusOK)
}

func handleDeleteUser(c *fiber.Ctx) error {
	if !requireAdminSettingsAccess(c) {
		return nil
	}

	targetUsername := c.Params("username")
	currentUsername := sessionUsername(c)
	if targetUsername == currentUsername {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Cannot delete the current user"})
	}

	cfg := config.GetConfig()
	user := cfg.FindUser(targetUsername)
	if user == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if user.Role == config.RoleAdmin && !user.Disabled && cfg.FindEnabledAdminCount() == 1 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Cannot remove the last enabled admin"})
	}

	updatedCfg := cloneConfig(cfg)
	newUsers := make([]config.User, 0, len(updatedCfg.Users)-1)
	for _, candidate := range updatedCfg.Users {
		if candidate.Username != targetUsername {
			newUsers = append(newUsers, candidate)
		}
	}
	updatedCfg.Users = newUsers

	if err := config.SaveConfig(updatedCfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save config"})
	}

	_ = servicelogs.LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       currentUsername,
		Action:     "user_removed",
		Method:     c.Method(),
		Route:      c.Path(),
		Parameters: json.RawMessage(fmt.Sprintf(`{"username":%q}`, targetUsername)),
		StatusCode: 200,
	})

	return c.SendStatus(fiber.StatusOK)
}

// handleGetUserOptions returns available apps and services for the user management UI.
func handleGetUserOptions(c *fiber.Ctx) error {
	if !requireAdminSettingsAccess(c) {
		return nil
	}

	cfg := config.GetConfig()

	realApps, err := dokku.GetDokkuApps(commandRunner)
	if err != nil {
		realApps = []string{}
		internallog.LogWarning(fmt.Sprintf("Error getting app list from dokku: %v", err))
	}

	appSet := make(map[string]bool)
	configApps := make(map[string]bool)
	for _, app := range realApps {
		appSet[app] = true
	}
	for _, user := range cfg.Users {
		for _, app := range user.Apps {
			if app != "*" {
				appSet[app] = true
				configApps[app] = true
			}
		}
	}
	apps := make([]string, 0, len(appSet))
	for app := range appSet {
		apps = append(apps, app)
	}
	sort.Strings(apps)

	// Use fast filesystem-based methods to get installed services and their instances
	services := make(map[string][]string)
	configServices := make(map[string]map[string]bool)

	// Get installed service plugins from filesystem (fast)
	installedServicePlugins, err := dokku.GetInstalledServicePluginsByFilesystem()
	if err != nil {
		internallog.LogWarning(fmt.Sprintf("Error getting installed service plugins from filesystem: %v", err))
		// Fallback to dokku plugin:list
		installedServicePlugins, err = dokku.GetAvailableServicePluginList(commandRunner)
		if err != nil {
			internallog.LogWarning(fmt.Sprintf("Error getting available service plugins: %v", err))
			installedServicePlugins = []string{}
		}
	}

	// For each installed plugin, get service instances from filesystem (fast)
	for _, svcType := range installedServicePlugins {
		svcNames, err := dokku.GetServiceNamesByFilesystem(svcType)
		if err != nil {
			internallog.LogWarning(fmt.Sprintf("Error getting service names for %s from filesystem: %v", svcType, err))
			// Fallback to dokku command
			svcNames, _ = dokku.GetServiceNamesOnly(dokku.DefaultCommandRunner, svcType)
		}
		services[svcType] = svcNames
		configServices[svcType] = make(map[string]bool)
	}

	// Also include services referenced in user config (may have been deleted)
	for _, user := range cfg.Users {
		for svcType, svcs := range user.Services {
			for _, svc := range svcs {
				if svc != "*" {
					if _, exists := services[svcType]; !exists {
						services[svcType] = []string{}
					}
					if _, exists := configServices[svcType]; !exists {
						configServices[svcType] = make(map[string]bool)
					}
					// Add to configServices tracking but don't duplicate in services list
					configServices[svcType][svc] = true
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"apps":                    apps,
		"configApps":              configApps,
		"services":                services,
		"configServices":          configServices,
		"installedServicePlugins": installedServicePlugins,
		"exportableServices":      installedServicePlugins,
	})
}

func buildUserFromPayload(req userPayload, requirePassword bool) (*config.User, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = config.RoleUser
	}
	if role != config.RoleAdmin && role != config.RoleUser {
		return nil, fmt.Errorf("invalid role")
	}

	user := &config.User{
		Username:          username,
		Role:              role,
		Routes:            dedupeStrings(req.Routes),
		Apps:              dedupeStrings(req.Apps),
		Services:          normalizeServiceSelections(req.Services),
		Disabled:          req.Disabled,
		CanCreateApps:     req.CanCreateApps,
		CanCreateServices: req.CanCreateServices,
	}
	githubUsername := strings.TrimSpace(req.GitHub.Username)
	if githubUsername != "" {
		user.GitHub = &config.UserGitHub{Username: githubUsername}
	}
	if requirePassword {
		if err := validatePasswordStrength(req.Password); err != nil {
			return nil, err
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password")
		}
		user.Password = string(hashedPassword)
	}
	user.Normalize()
	return user, nil
}

func validateUserMutation(cfg *config.Config, targetUsername string, updatedUser *config.User, currentUsername string) error {
	targetUser := cfg.FindUser(targetUsername)
	if targetUser == nil {
		return fmt.Errorf("user not found")
	}

	if targetUsername == currentUsername && updatedUser.Disabled {
		return fmt.Errorf("cannot disable the current user")
	}
	if targetUsername == currentUsername && updatedUser.Role != config.RoleAdmin && targetUser.Role == config.RoleAdmin && cfg.FindEnabledAdminCount() == 1 {
		return fmt.Errorf("cannot remove the last enabled admin")
	}
	if targetUser.Role == config.RoleAdmin && !targetUser.Disabled && (updatedUser.Role != config.RoleAdmin || updatedUser.Disabled) && cfg.FindEnabledAdminCount() == 1 {
		return fmt.Errorf("cannot remove the last enabled admin")
	}
	return nil
}

func requireAdminSettingsAccess(c *fiber.Ctx) bool {
	if sessionRole(c) == config.RoleAdmin {
		return true
	}
	_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Administrator access is required"})
	return false
}

func sessionRole(c *fiber.Ctx) string {
	sess, _ := middleware.GetStore().Get(c)
	role, _ := sess.Get("role").(string)
	if role == "" {
		role, _ = sess.Get("auth_type").(string)
	}
	return role
}

func updateCurrentSessionUsername(c *fiber.Ctx, username, role string) error {
	sess, err := middleware.GetStore().Get(c)
	if err != nil {
		return err
	}
	sess.Set("username", username)
	sess.Set("user", username)
	sess.Set("role", role)
	sess.Set("auth_type", role)
	return sess.Save()
}

func sessionUsername(c *fiber.Ctx) string {
	sess, _ := middleware.GetStore().Get(c)
	username, _ := sess.Get("username").(string)
	if username == "" {
		username, _ = sess.Get("user").(string)
	}
	if username == "" {
		return "unknown"
	}
	return username
}

func servicesOrEmpty(services map[string][]string) map[string][]string {
	if len(services) == 0 {
		return map[string][]string{}
	}
	return services
}

func sliceOrEmpty(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return values
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeServiceSelections(services map[string][]string) map[string][]string {
	if len(services) == 0 {
		return nil
	}
	normalized := make(map[string][]string, len(services))
	for svcType, names := range services {
		if deduped := dedupeStrings(names); len(deduped) > 0 {
			normalized[strings.TrimSpace(svcType)] = deduped
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func validatePasswordStrength(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return errors.New(strongPasswordError)
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSymbol := false

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			if !unicode.IsSpace(r) {
				hasSymbol = true
			}
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSymbol {
		return errors.New(strongPasswordError)
	}

	return nil
}

func cloneConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return &config.Config{}
	}

	cloned := *cfg
	if len(cfg.Users) > 0 {
		cloned.Users = make([]config.User, len(cfg.Users))
		for i, user := range cfg.Users {
			cloned.Users[i] = cloneUser(user)
		}
	}
	return &cloned
}

func cloneUser(user config.User) config.User {
	cloned := user
	if len(user.Routes) > 0 {
		cloned.Routes = append([]string(nil), user.Routes...)
	}
	if len(user.Apps) > 0 {
		cloned.Apps = append([]string(nil), user.Apps...)
	}
	if len(user.Services) > 0 {
		cloned.Services = make(map[string][]string, len(user.Services))
		for svcType, names := range user.Services {
			cloned.Services[svcType] = append([]string(nil), names...)
		}
	}
	if user.GitHub != nil {
		github := *user.GitHub
		cloned.GitHub = &github
	}
	return cloned
}

func userRequestError(c *fiber.Ctx, err error) error {
	message := err.Error()
	status := fiber.StatusBadRequest
	if message == "User already exists" {
		status = fiber.StatusConflict
	}
	return c.Status(status).JSON(fiber.Map{"error": message})
}
