package dokku

import (
	"fmt"
	"os"
	"strings"

	"github.com/pruvon/pruvon/internal/models"
)

// ParseEnvVars extracts environment variables from Dokku output
func ParseEnvVars(output string) []models.EnvVar {
	var vars []models.EnvVar
	lines := strings.Split(output, "\n")
	inVarsSection := false

	for _, line := range lines {
		// Trim spaces from line
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check if we're entering the env vars section
		if strings.Contains(line, "env vars") {
			inVarsSection = true
			continue
		}

		// Skip separator lines
		if strings.Contains(line, "=====") {
			continue
		}

		// Only process lines after the env vars header
		if inVarsSection && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if key != "" {
					vars = append(vars, models.EnvVar{
						Key:   key,
						Value: value,
					})
				}
			}
		}
	}

	return vars
}

// ParsePorts extracts port mappings from Dokku output
func ParsePorts(output string) []models.PortMapping {
	var mappings []models.PortMapping
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Ports map:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				portList := strings.TrimSpace(parts[1])
				for _, mapping := range strings.Fields(portList) {
					parts := strings.Split(mapping, ":")
					if len(parts) == 3 {
						mappings = append(mappings, models.PortMapping{
							Protocol:  parts[0],
							Host:      parts[1],
							Container: parts[2],
						})
					}
				}
			}
		}
	}
	return mappings
}

// ParseProcessInfo extracts process information from Dokku output
func ParseProcessInfo(output string) []models.ProcessInfo {
	var info []models.ProcessInfo
	lines := strings.Split(output, "\n")
	processTypes := make(map[string]int) // Her process tipi için sayı tutacak

	for _, line := range lines {
		if strings.Contains(line, ":") && !strings.Contains(line, "=====") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Add Procfile path info if found
				if strings.Contains(name, "Ps computed procfile path") {
					info = append(info, models.ProcessInfo{
						Name:  "ProcfilePath",
						Value: value,
					})
				}

				// Add app.json path info if found
				if strings.Contains(name, "App JSON computed appjson path") {
					info = append(info, models.ProcessInfo{
						Name:  "AppJsonPath",
						Value: value,
					})
				}

				// Status ile başlayan satırları özel olarak işle
				if strings.HasPrefix(name, "Status ") {
					// Process tipini ve numarasını ayır (örn: "Status web 1" -> "web")
					nameParts := strings.Fields(name)
					if len(nameParts) >= 3 {
						processType := nameParts[1]
						isRunning := strings.Contains(value, "running") // Fix: contains -> Contains

						// Process tipinin sayısını artır
						if _, exists := processTypes[processType]; !exists {
							processTypes[processType] = 0
						}
						if isRunning {
							processTypes[processType]++
						}
					}
				}
			}
		}
	}

	// Her process tipi için bilgileri oluştur
	for processType, count := range processTypes {
		var status string
		if count > 0 {
			status = "running"
		} else {
			status = "stopped"
		}

		info = append(info, models.ProcessInfo{
			Name:  "Status " + processType,
			Value: fmt.Sprintf("%s (%d)", status, count),
		})
	}

	return info
}

// ParseNginxConfig extracts Nginx configuration from Dokku output
func ParseNginxConfig(output string) []models.NginxConfig {
	var configs []models.NginxConfig
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Nginx computed client max body size:") {
			value := strings.TrimSpace(strings.Split(line, ":")[1])
			configs = append(configs, models.NginxConfig{
				Name:  "client-max-body-size",
				Value: value,
			})
		}
		if strings.Contains(line, "Nginx computed proxy read timeout:") {
			value := strings.TrimSpace(strings.Split(line, ":")[1])
			configs = append(configs, models.NginxConfig{
				Name:  "proxy-read-timeout",
				Value: value,
			})
		}
	}
	return configs
}

// GetServiceConfig retrieves custom configuration for a service such as custom image and image version
func GetServiceConfig(runner CommandRunner, serviceType string, serviceName string) (map[string]string, error) {
	result := make(map[string]string)

	// Get the service info
	output, err := runner.RunCommand("dokku", fmt.Sprintf("%s:info", serviceType), serviceName)
	if err != nil {
		return result, err
	}

	// Look for image information in the output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for image information
		if strings.Contains(line, "Image:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				imageInfo := strings.TrimSpace(parts[1])

				// Check if it has both image and version (image:version format)
				if strings.Contains(imageInfo, ":") {
					imageParts := strings.Split(imageInfo, ":")
					if len(imageParts) > 1 {
						result["image"] = imageParts[0]
						result["image_version"] = imageParts[1]
					}
				} else {
					// Just image without version
					result["image"] = imageInfo
				}
			}
		}
	}

	return result, nil
}

// ReadCronSetting reads a global cron setting (mailfrom or mailto)
func ReadCronSetting(settingName string) (string, error) {
	if settingName != "mailfrom" && settingName != "mailto" {
		return "", fmt.Errorf("invalid cron setting name: %s", settingName)
	}

	path := fmt.Sprintf("/var/lib/dokku/config/cron/--global/%s", settingName)

	// For testing/development, use /tmp/cron-setting-{settingName} if the real file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		tmpPath := fmt.Sprintf("/tmp/cron-setting-%s", settingName)
		if _, err := os.Stat(tmpPath); err == nil {
			path = tmpPath
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read cron %s setting: %v", settingName, err)
	}

	return strings.TrimSpace(string(content)), nil
}

// WriteCronSetting writes a value to a global cron setting (mailfrom or mailto)
func WriteCronSetting(settingName, content string) error {
	if settingName != "mailfrom" && settingName != "mailto" {
		return fmt.Errorf("invalid cron setting name: %s", settingName)
	}

	path := fmt.Sprintf("/var/lib/dokku/config/cron/--global/%s", settingName)

	// For testing/development, use /tmp/cron-setting-{settingName} if the real file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dirPath := "/var/lib/dokku/config/cron/--global"
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			tmpPath := fmt.Sprintf("/tmp/cron-setting-%s", settingName)
			// Create the tmp file if it doesn't exist yet
			if _, err := os.Stat(tmpPath); err == nil || os.IsNotExist(err) {
				path = tmpPath
			}
		} else {
			// Create the directory if it doesn't exist
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory for cron settings: %v", err)
			}
		}
	}

	// Check if we're clearing the setting (empty content)
	if content == "" {
		// If the file exists, remove it
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove cron %s setting: %v", settingName, err)
			}
		}
		return nil
	}

	// Get file info to preserve permissions if file exists
	var perm os.FileMode = 0644
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	// Write the content to the file
	err := os.WriteFile(path, []byte(content), perm)
	if err != nil {
		return fmt.Errorf("failed to write cron %s setting: %v", settingName, err)
	}

	return nil
}
