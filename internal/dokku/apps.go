package dokku

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pruvon/pruvon/internal/models"

	"github.com/gofiber/fiber/v2"
)

// GetDokkuApps returns a list of all Dokku apps
func GetDokkuApps(runner CommandRunner) ([]string, error) {
	output, err := runner.RunCommand("dokku", "apps:list")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	var apps []string
	for _, line := range lines {
		if line != "" && !strings.Contains(line, "=====") {
			apps = append(apps, line)
		}
	}
	return apps, nil
}

// GetAppContainers returns a list of Docker containers for the specified app
func GetAppContainers(runner CommandRunner, appName string) ([]models.Container, error) {
	output, err := runner.RunCommand("docker", "ps", "--format", "{{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}")
	if err != nil {
		return nil, fmt.Errorf("container list could not be retrieved: %v", err)
	}
	lines := strings.Split(output, "\n")
	var containers []models.Container
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}

		containerName := parts[5]
		// Dokku container naming format: <app-name>.<process-type>.<index>
		// or for more precise filtering: starts with <app-name>. or exactly <app-name>
		if strings.HasPrefix(containerName, appName+".") || containerName == appName {
			container := models.Container{
				ID:      parts[0],
				Image:   parts[1],
				Command: parts[2],
				Status:  parts[3],
				Ports:   parts[4],
				Name:    parts[5],
			}
			containers = append(containers, container)
		}
	}
	return containers, nil
}

// GetAppCronJobs returns a list of cron jobs for the specified app
func GetAppCronJobs(runner CommandRunner, appName string) ([]models.CronJob, error) {
	output, err := runner.RunCommand("dokku", "cron:list", "--format", "json", appName)
	if err != nil {
		return nil, fmt.Errorf("cron jobs could not be retrieved: %v", err)
	}

	var jobs []models.CronJob
	if err := json.Unmarshal([]byte(output), &jobs); err != nil {
		return nil, fmt.Errorf("cron jobs could not be parsed: %v", err)
	}
	return jobs, nil
}

// GetAppJson returns the app.json content for the specified app
func GetAppJson(runner CommandRunner, appName string) (string, error) {
	filePath := fmt.Sprintf("/var/lib/dokku/data/app-json/%s/app.json", appName)
	content, err := OsReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(content), nil
}

// GetAppCreatedAt returns the creation time of the specified app
func GetAppCreatedAt(runner CommandRunner, appName string) string {
	filepath := fmt.Sprintf("/var/lib/dokku/config/apps/%s/created-at", appName)
	content, err := OsReadFile(filepath)
	if err != nil {
		return ""
	}

	// Convert unix timestamp to time.Time
	timestampStr := strings.TrimSpace(string(content))
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return ""
	}

	// Format the time with new format
	t := time.Unix(timestamp, 0)
	return t.Format("02.01.2006 15:04")
}

// GetLastDeployTime returns the last deployment time of the specified app
func GetLastDeployTime(runner CommandRunner, appName string) string {
	filePath := fmt.Sprintf("/var/lib/dokku/data/app-json/%s/app.json", appName)
	fileInfo, err := OsStat(filePath)
	if err != nil {
		return ""
	}

	// Format the modification time
	return fileInfo.ModTime().Format("02.01.2006 15:04")
}

// GetProcfileHandler handles requests for Procfile content
func GetProcfileHandler(c *fiber.Ctx) error {
	appName := c.Params("app")
	filePath := fmt.Sprintf("/var/lib/dokku/data/ps/%s/Procfile", appName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(fiber.Map{"content": ""})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read Procfile"})
	}
	return c.JSON(fiber.Map{"content": string(content)})
}

// GetAppLogs returns the logs for the specified app
func GetAppLogs(runner CommandRunner, appName string, num int) (string, error) {
	output, err := runner.RunCommand("dokku", "logs", appName, "--num", strconv.Itoa(num))
	if err != nil {
		return "", fmt.Errorf("application logs could not be retrieved: %v", err)
	}
	return output, nil
}
