package api

import (
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/docker"
	"github.com/pruvon/pruvon/internal/models"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// SetupDockerRoutes registers all Docker-related routes
func SetupDockerRoutes(app *fiber.App) {
	app.Get("/api/docker/stats", handleDockerStats)
	app.Post("/api/docker/prune", handleDockerPrune)
	app.Get("/api/docker/containers", handleListContainers)
	app.Get("/api/docker/containers/:id/logs", handleContainerLogs)
	app.Get("/api/docker/containers/:id/logs/stream", handleContainerLogsStream)
	app.Post("/api/docker/containers/:id/start", handleStartContainer)
	app.Post("/api/docker/containers/:id/stop", handleStopContainer)
	app.Post("/api/docker/containers/:id/restart", handleRestartContainer)
	app.Delete("/api/docker/containers/:id", handleRemoveContainer)
}

// handleDockerStats returns statistics about Docker (moved from others.go)
func handleDockerStats(c *fiber.Ctx) error {
	stats, err := docker.GetDockerStats(commandRunner)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Docker statistics could not be retrieved: %v", err),
		})
	}
	return c.JSON(stats)
}

// handleDockerPrune removes unused Docker resources (moved from others.go)
func handleDockerPrune(c *fiber.Ctx) error {
	_, err := commandRunner.RunCommand("docker", "system", "prune", "--force")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Docker cleanup failed: %v", err),
		})
	}
	return c.SendStatus(fiber.StatusOK)
}

// handleListContainers lists all Docker containers with detailed information
func handleListContainers(c *fiber.Ctx) error {
	// Use a more detailed format to get all container information
	output, err := commandRunner.RunCommand("docker", "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to list containers: %v", err),
		})
	}

	// Parse the output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	containers := make([]models.Container, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		var container models.Container
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to parse container information: %v", err),
			})
		}

		// Get additional port details if ports are present
		if container.Ports != "" && container.Ports != "N/A" {
			// Get full container details to ensure we have all information
			detailOutput, err := commandRunner.RunCommand("docker", "inspect", "--format", "{{json .}}", container.ID)
			if err == nil {
				var containerDetail map[string]interface{}
				if err := json.Unmarshal([]byte(detailOutput), &containerDetail); err == nil {
					// Extract container name (remove leading slash)
					if name, ok := containerDetail["Name"].(string); ok && name != "" {
						container.Name = strings.TrimPrefix(name, "/")
					}

					// Extract network ports with binding details
					if networkSettings, ok := containerDetail["NetworkSettings"].(map[string]interface{}); ok {
						if ports, ok := networkSettings["Ports"].(map[string]interface{}); ok {
							portDetails := make([]string, 0)
							for containerPort, bindings := range ports {
								if bindings != nil {
									if bindingsArray, ok := bindings.([]interface{}); ok && len(bindingsArray) > 0 {
										for _, binding := range bindingsArray {
											if bindingMap, ok := binding.(map[string]interface{}); ok {
												hostIP := bindingMap["HostIp"].(string)
												if hostIP == "0.0.0.0" || hostIP == "" {
													hostIP = "*"
												}
												hostPort := bindingMap["HostPort"].(string)
												portDetails = append(portDetails, fmt.Sprintf("%s:%s->%s", hostIP, hostPort, containerPort))
											}
										}
									}
								} else {
									// For exposed ports (not published), show as 0.0.0.0:port
									portParts := strings.Split(containerPort, "/")
									if len(portParts) > 0 {
										portNumber := portParts[0]
										portDetails = append(portDetails, fmt.Sprintf("0.0.0.0:%s->%s", portNumber, containerPort))
									} else {
										portDetails = append(portDetails, containerPort)
									}
								}
							}
							if len(portDetails) > 0 {
								container.PortDetails = strings.Join(portDetails, ", ")
							}
						}
					}
				}
			}
		}

		containers = append(containers, container)
	}

	return c.JSON(containers)
}

// handleContainerLogs returns logs for a specific container
func handleContainerLogs(c *fiber.Ctx) error {
	containerId := c.Params("id")
	if containerId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	// Get the number of log lines to return
	lines := c.Query("lines", "100")

	// Get container logs
	output, err := commandRunner.RunCommand("docker", "logs", "--tail", lines, containerId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get container logs: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"logs": output,
	})
}

// handleContainerLogsStream sets up WebSocket connection for streaming container logs
func handleContainerLogsStream(c *fiber.Ctx) error {
	// This is a placeholder - actual WebSocket handling will be done in routes/ws/docker.go
	return c.SendStatus(fiber.StatusNotImplemented)
}

// handleStartContainer starts a stopped container
func handleStartContainer(c *fiber.Ctx) error {
	containerId := c.Params("id")
	if containerId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	_, err := commandRunner.RunCommand("docker", "start", containerId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to start container: %v", err),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// handleStopContainer stops a running container
func handleStopContainer(c *fiber.Ctx) error {
	containerId := c.Params("id")
	if containerId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	_, err := commandRunner.RunCommand("docker", "stop", containerId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to stop container: %v", err),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// handleRestartContainer restarts a container
func handleRestartContainer(c *fiber.Ctx) error {
	containerId := c.Params("id")
	if containerId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	_, err := commandRunner.RunCommand("docker", "restart", containerId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to restart container: %v", err),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}

// handleRemoveContainer removes a container
func handleRemoveContainer(c *fiber.Ctx) error {
	containerId := c.Params("id")
	if containerId == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Container ID is required",
		})
	}

	// Check if force parameter is provided
	force := c.Query("force", "false")
	args := []string{"rm"}
	if force == "true" {
		args = append(args, "-f")
	}
	args = append(args, containerId)

	_, err := commandRunner.RunCommand("docker", args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to remove container: %v", err),
		})
	}

	return c.SendStatus(fiber.StatusOK)
}
