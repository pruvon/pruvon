package api

import (
	"fmt"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/system"

	"github.com/gofiber/fiber/v2"
)

func SetupOthersRoutes(app *fiber.App) {
	app.Get("/api/metrics", handleMetrics)
	app.Get("/api/metrics/resources", handleResourceMetrics)
	app.Get("/api/metrics/counts", handleCountMetrics)
	app.Get("/api/server/info", handleServerInfo)
	app.Get("/api/server/domain", handleServerDomain)
	app.Get("/api/version", handleVersion)
}

// handleVersion returns the current version of Pruvon
func handleVersion(c *fiber.Ctx) error {
	// Get version from locals which is set by middleware in main.go
	version := c.Locals("version")
	if version == nil {
		version = "Unknown" // Fallback if version is not set
	}

	return c.JSON(fiber.Map{
		"version": version,
	})
}

func handleMetrics(c *fiber.Ctx) error {
	metrics := system.GetSystemMetrics()
	return c.JSON(metrics)
}

func handleResourceMetrics(c *fiber.Ctx) error {
	metrics := system.GetSystemMetrics()
	return c.JSON(fiber.Map{
		"cpu_usage":  metrics.CPUUsage,
		"ram_usage":  metrics.RAMUsage,
		"disk_usage": metrics.DiskUsage,
		"cpu_info":   metrics.CPUInfo,
		"ram_info":   metrics.RAMInfo,
		"disk_info":  metrics.DiskInfo,
		"load_avg":   metrics.LoadAvg,
		"swap_info":  metrics.SwapInfo,
	})
}

func handleCountMetrics(c *fiber.Ctx) error {
	metrics := system.GetSystemMetrics()
	return c.JSON(fiber.Map{
		"app_count":       metrics.AppCount,
		"service_count":   metrics.ServiceCount,
		"container_count": metrics.ContainerCount,
	})
}

func handleServerInfo(c *fiber.Ctx) error {
	return c.JSON(system.GetServerInfo())
}

func handleServerDomain(c *fiber.Ctx) error {
	domain, err := dokku.GetServerDomain(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Server domain information could not be retrieved: %v", err),
		})
	}
	return c.JSON(fiber.Map{
		"domain": domain,
	})
}
