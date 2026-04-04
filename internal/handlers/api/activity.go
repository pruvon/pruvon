package api

import (
	"github.com/pruvon/pruvon/internal/handlers/common"
	"github.com/pruvon/pruvon/internal/services/logs"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ActivityLog represents a logged user activity
type ActivityLog struct {
	Type      string    `json:"type"`
	Service   string    `json:"service"`
	Stack     string    `json:"stack"`
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
}

var activityLogs []ActivityLog

// SetupActivityRoutes configures routes for activity logging
func SetupActivityRoutes(app *fiber.App) {
	// Log activity
	app.Post("/api/activity/log", func(c *fiber.Ctx) error {
		var log ActivityLog
		if err := c.BodyParser(&log); err != nil {
			return common.ErrorResponse(c, fiber.StatusBadRequest, "Invalid request body")
		}

		// Add additional info
		log.IP = c.IP()
		log.UserAgent = c.Get("User-Agent")
		if log.Timestamp.IsZero() {
			log.Timestamp = time.Now()
		}

		// Store the log
		activityLogs = append(activityLogs, log)

		// Log to server log as well
		_ = logs.LogWithParams(c, "activity_log", fiber.Map{
			"type":    log.Type,
			"service": log.Service,
			"stack":   log.Stack,
			"command": log.Command,
		})

		return common.SuccessResponse(c, fiber.Map{"status": "success"})
	})

	// Get all activity logs
	app.Get("/api/activity/logs", func(c *fiber.Ctx) error {
		return common.SuccessResponse(c, activityLogs)
	})
}
