package server

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pruvon/pruvon/internal/services/update"

	"github.com/gofiber/fiber/v2"
)

type cachedUpdateState struct {
	info        update.UpdateInfo
	err         error
	checkedAt   time.Time
	initialized bool
	refreshing  bool
}

var (
	updateStateMu   sync.RWMutex
	updateState     cachedUpdateState
	updateCheckTTL  = 15 * time.Minute
	updateCheckNow  = time.Now
	updateCheckFunc = update.CheckForUpdates
)

// SetupVersionMiddleware sets up middleware to inject version information into templates
func SetupVersionMiddleware(app *fiber.App, version string) {
	app.Use(func(c *fiber.Ctx) error {
		// Set version explicitly for all templates
		c.Locals("version", version)
		return c.Next()
	})
}

// SetupUpdateCheckerMiddleware sets up middleware to check for updates
func SetupUpdateCheckerMiddleware(app *fiber.App, version string) {
	app.Use(func(c *fiber.Ctx) error {
		applyCachedUpdateLocals(c)
		if shouldRefreshUpdateInfo(c) {
			refreshUpdateInfoAsync(version)
		}
		return c.Next()
	})
}

func applyCachedUpdateLocals(c *fiber.Ctx) {
	updateStateMu.RLock()
	state := updateState
	updateStateMu.RUnlock()

	if !state.initialized {
		c.Locals("updateCheckError", true)
		c.Locals("updateAvailable", false)
		c.Locals("latestVersion", "")
		return
	}

	if state.err != nil {
		c.Locals("updateCheckError", true)
		c.Locals("updateAvailable", false)
		c.Locals("latestVersion", "")
		return
	}

	c.Locals("updateCheckError", false)
	c.Locals("updateAvailable", state.info.UpdateAvailable)
	c.Locals("latestVersion", state.info.LatestVersion)
}

func shouldRefreshUpdateInfo(c *fiber.Ctx) bool {
	if c.Method() != fiber.MethodGet {
		return false
	}

	path := c.Path()
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") || strings.HasPrefix(path, "/static/") {
		return false
	}

	return true
}

func refreshUpdateInfoAsync(version string) {
	now := updateCheckNow()

	updateStateMu.Lock()
	if updateState.refreshing || (updateState.initialized && now.Sub(updateState.checkedAt) < updateCheckTTL) {
		updateStateMu.Unlock()
		return
	}
	updateState.refreshing = true
	updateStateMu.Unlock()

	go func() {
		info, err := updateCheckFunc(version)
		checkedAt := updateCheckNow()

		updateStateMu.Lock()
		updateState.info = info
		updateState.err = err
		updateState.checkedAt = checkedAt
		updateState.initialized = true
		updateState.refreshing = false
		shouldLog := err == nil && info.UpdateAvailable
		currentVersion := info.CurrentVersion
		latestVersion := info.LatestVersion
		updateStateMu.Unlock()

		if shouldLog {
			fmt.Printf("Update available! Current: v%s, Latest: v%s\n", currentVersion, latestVersion)
		}
	}()
}

func resetUpdateCheckState() {
	updateStateMu.Lock()
	defer updateStateMu.Unlock()
	updateState = cachedUpdateState{}
}
