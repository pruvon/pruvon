package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pruvon/pruvon/internal/exec"
	"github.com/pruvon/pruvon/internal/models"
)

// CommandRunner interface for command execution
type CommandRunner = exec.CommandRunner

// DefaultCommandRunner is the default command runner instance
var DefaultCommandRunner CommandRunner = exec.NewCommandRunner()

// OsStat is used to replace os.Stat for testing
var OsStat = os.Stat

// GetDockerStats returns Docker statistics
func GetDockerStats(runner CommandRunner) (models.DockerStats, error) {
	var info models.DockerInfo
	output, err := runner.RunCommand("docker", "info", "--format", "{{json .}}")
	if err != nil {
		return models.DockerStats{}, fmt.Errorf("docker info could not be retrieved: %v", err)
	}

	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return models.DockerStats{}, fmt.Errorf("docker info could not be parsed: %v", err)
	}

	return models.DockerStats{
		Version:           info.ServerVersion,
		RunningContainers: info.ContainersRunning,
		TotalContainers:   info.Containers,
		TotalImages:       info.Images,
	}, nil
}

// getAppContainers returns the list of containers for the specified app
func getAppContainers(runner CommandRunner, appName string) ([]models.Container, error) {
	output, err := runner.RunCommand("docker", "ps", "--format", "{{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}")
	if err != nil {
		return nil, fmt.Errorf("container list could not be retrieved: %v", err)
	}

	var containers []models.Container
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}
		// Filter by app name
		if !strings.Contains(parts[5], appName) {
			continue
		}
		containers = append(containers, models.Container{
			ID:      parts[0],
			Image:   parts[1],
			Command: parts[2],
			Status:  parts[3],
			Ports:   parts[4],
			Name:    parts[5],
		})
	}
	return containers, nil
}

// GetContainerStats returns resource usage statistics for the specified app's containers
func GetContainerStats(runner CommandRunner, appName string) models.ContainerStats {
	stats := models.ContainerStats{}

	// Check if app is deployed by looking for its docker container
	containers, err := getAppContainers(runner, appName)
	if err != nil {
		return models.ContainerStats{
			CPUUsage:      0,
			MemoryUsage:   0,
			DiskUsage:     0,
			DiskUsageText: "0B/0B",
			IsDeployed:    false,
		}
	}
	stats.IsDeployed = len(containers) > 0

	// If not deployed, return early with default values
	if !stats.IsDeployed {
		return models.ContainerStats{
			CPUUsage:      0,
			MemoryUsage:   0,
			DiskUsage:     0,
			DiskUsageText: "0B/0B",
		}
	}

	// Get stats for first container
	container := containers[0]
	output, err := runner.RunCommand("docker", "stats", "--no-stream", "--format",
		"{{.CPUPerc}}\t{{.MemPerc}}", container.ID)
	if err != nil {
		return models.ContainerStats{
			CPUUsage:      0,
			MemoryUsage:   0,
			DiskUsage:     0,
			DiskUsageText: "0B/0B",
			IsDeployed:    true,
		}
	}

	if output != "" {
		fields := strings.Fields(output)
		if len(fields) >= 2 {
			cpuStr := strings.TrimSuffix(strings.TrimSpace(fields[0]), "%")
			cpu, _ := strconv.ParseFloat(cpuStr, 64)
			stats.CPUUsage = cpu

			memStr := strings.TrimSuffix(strings.TrimSpace(fields[1]), "%")
			mem, _ := strconv.ParseFloat(memStr, 64)
			stats.MemoryUsage = mem
		}
	}

	// Calculate disk usage
	appStorage := fmt.Sprintf("/var/lib/dokku/data/storage/%s", appName)

	// Default to 0/0 if storage directory doesn't exist
	if _, err := OsStat(appStorage); os.IsNotExist(err) {
		stats.DiskUsage = 0
		stats.DiskUsageText = "0B/0B"
		return stats
	}

	// Get used space
	diskUsageOut, err := runner.RunCommand("du", "-sb", appStorage)
	var usedBytes int64
	if err == nil {
		parts := strings.Fields(diskUsageOut)
		if len(parts) > 0 {
			usedBytes, _ = strconv.ParseInt(parts[0], 10, 64)
		}
	}

	// Get total space
	dfOut, err := runner.RunCommand("df", "-B1", appStorage)
	var totalBytes int64
	if err == nil {
		lines := strings.Split(dfOut, "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) > 1 {
				totalBytes, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}

	// Calculate disk usage percentage and human-readable format
	if totalBytes > 0 {
		stats.DiskUsage = (float64(usedBytes) / float64(totalBytes)) * 100
		stats.DiskUsage = float64(int(stats.DiskUsage*100)) / 100 // Round to 2 decimal places
	}

	usedHuman, _ := runner.RunCommand("du", "-sh", appStorage)
	totalHuman, _ := runner.RunCommand("df", "-h", appStorage)

	// Handle empty or invalid output
	usedSize := "0B"
	if len(strings.Fields(usedHuman)) > 0 {
		usedSize = strings.Fields(usedHuman)[0]
	}

	totalSize := "0B"
	dfLines := strings.Split(totalHuman, "\n")
	if len(dfLines) > 1 && len(strings.Fields(dfLines[1])) > 1 {
		totalSize = strings.Fields(dfLines[1])[1]
	}

	stats.DiskUsageText = fmt.Sprintf("%s/%s", usedSize, totalSize)

	return stats
}
