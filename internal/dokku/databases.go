package dokku

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pruvon/pruvon/internal/models"
)

// GetDatabases returns a list of databases of the specified type
func GetDatabases(runner CommandRunner, dbType string) ([]models.Database, error) {
	var cmd string
	switch dbType {
	case "postgres":
		cmd = "postgres:list"
	case "mariadb":
		cmd = "mariadb:list"
	case "mongo":
		cmd = "mongo:list"
	case "redis":
		cmd = "redis:list"
	default:
		return nil, fmt.Errorf("invalid database type: %s", dbType)
	}

	output, err := runner.RunCommand("dokku", cmd)
	if err != nil {
		return nil, fmt.Errorf("database list could not be retrieved: %v", err)
	}

	lines := strings.Split(output, "\n")
	var dbs []models.Database
	for _, line := range lines {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		name := strings.TrimSpace(line)
		dbs = append(dbs, models.Database{
			Name: name,
		})
	}

	// Get status information for each database by running the <service>:list command
	// This will get detailed info for all services of this type in one command
	_, err = runner.RunCommand("dokku", dbType+":list")
	if err == nil {
		// Parse the detailed status output
		for i := range dbs {
			// Run info command for each individual service to get detailed status
			svcInfoOutput, err := runner.RunCommand("dokku", dbType+":info", dbs[i].Name)
			if err == nil {
				// Find status of this specific service
				status := parseServiceStatus(svcInfoOutput)
				dbs[i].Status = status

				// Also try to extract version information
				version := parseServiceVersion(svcInfoOutput)
				if version != "" {
					dbs[i].Version = version
				}
			}
		}
	}

	return dbs, nil
}

// GetDatabaseInfo returns detailed information about a database
func GetDatabaseInfo(runner CommandRunner, dbType string, dbName string) (models.ServiceInstanceInfo, error) {
	info := models.ServiceInstanceInfo{
		Name:         dbName,
		Service:      dbType,
		InstanceName: dbName,
		Details:      make(map[string]string),
	}

	// Set type to the actual database type
	info.Type = dbType

	var output string
	var err error

	switch dbType {
	case "postgres":
		output, err = runner.RunCommand("dokku", "postgres:info", dbName)
	case "mariadb":
		output, err = runner.RunCommand("dokku", "mariadb:info", dbName)
	case "mongo":
		output, err = runner.RunCommand("dokku", "mongo:info", dbName)
	case "redis":
		output, err = runner.RunCommand("dokku", "redis:info", dbName)
	default:
		return info, fmt.Errorf("invalid database type: %s", dbType)
	}

	if err != nil {
		return info, fmt.Errorf("database information could not be retrieved: %v", err)
	}

	lines := strings.Split(output, "\n")
	// Currently linkedApps is unused, might be needed in the future
	// var linkedApps []string
	var url string
	var version string
	var status string
	var internalIP string
	var containerID string

	for _, line := range lines {
		// Skip empty lines and separator lines
		if line == "" || strings.HasPrefix(line, "===") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Skip empty or "-" values
		if value == "-" || value == "" {
			continue
		}

		// Extract specific information
		if strings.Contains(key, "DSN") || strings.Contains(key, "URL") || strings.Contains(key, "Dsn") {
			// Get the connection URL
			url = value
		} else if strings.Contains(key, "version") || strings.Contains(key, "Version") {
			// Get version information
			version = value
		} else if strings.Contains(key, "Status") || strings.Contains(key, "status") {
			// Get status information
			status = value
		} else if strings.Contains(strings.ToLower(key), "internal ip") || strings.Contains(strings.ToLower(key), "internal_ip") {
			// Get internal IP information
			internalIP = value
		} else if strings.Contains(strings.ToLower(key), "container") && !strings.Contains(strings.ToLower(key), "mount") {
			// Get container ID - usually in format "Container: a72f95864f43" or similar
			// Make sure this is not a container mount path
			if strings.Contains(value, " ") {
				parts := strings.Fields(value)
				if len(parts) > 0 {
					containerID = parts[0]
				}
			} else {
				containerID = value
			}
		} else if key == "Id" {
			// Get container ID from the "Id" field
			containerID = value
		}
	}

	// Update the struct with the extracted information
	info.URL = url
	info.Version = version
	info.Status = status
	info.ContainerID = containerID

	// Add internal IP to the details map if available
	if internalIP != "" {
		if info.Details == nil {
			info.Details = make(map[string]string)
		}
		info.Details["Internal IP"] = internalIP
	}

	// Fetch resource limits from Docker inspect if we have a container ID
	if containerID != "" {
		inspectOutput, err := runner.RunCommand("docker", "inspect", containerID)
		if err == nil {
			// Parse the JSON output from docker inspect
			var containerDetails []map[string]interface{}
			if err := json.Unmarshal([]byte(inspectOutput), &containerDetails); err == nil && len(containerDetails) > 0 {
				if hostConfig, ok := containerDetails[0]["HostConfig"].(map[string]interface{}); ok {
					// Extract CPU and memory limits
					resourceLimits := &models.ResourceLimits{}

					// Get NanoCpus and convert to readable format (divide by 1,000,000,000)
					if nanoCpus, ok := hostConfig["NanoCpus"].(float64); ok && nanoCpus > 0 {
						cpuValue := nanoCpus / 1000000000.0
						resourceLimits.CPU = fmt.Sprintf("%.2f", cpuValue)
					}

					// Get Memory limit and convert to human-readable format
					if memoryBytes, ok := hostConfig["Memory"].(float64); ok && memoryBytes > 0 {
						// Format memory as MB or GB based on size
						if memoryBytes >= 1024*1024*1024 {
							// If memory is >= 1GB, format as GB
							memoryGB := memoryBytes / (1024 * 1024 * 1024)
							resourceLimits.Memory = fmt.Sprintf("%.0fG", memoryGB)
						} else {
							// Otherwise format as MB
							memoryMB := memoryBytes / (1024 * 1024)
							resourceLimits.Memory = fmt.Sprintf("%.0fM", memoryMB)
						}
					}

					// Only set resource limits if at least one value is present
					if resourceLimits.CPU != "" || resourceLimits.Memory != "" {
						info.ResourceLimits = resourceLimits
					}
				}
			}
		}
	}

	return info, nil
}

// ExportDatabase exports a database to a file and returns the filename
func ExportDatabase(runner CommandRunner, dbType string, dbName string) (string, error) {
	currentTime := time.Now().Format("2006-01-02-15-04-05")

	// Determine file extension based on database type
	var extension string
	switch dbType {
	case "postgres":
		extension = "sql"
	case "mariadb":
		extension = "sql"
	case "mongo":
		extension = "archive"
	case "redis":
		extension = "rdb"
	default:
		return "", fmt.Errorf("unsupported database type: %s", dbType)
	}

	// Remove underscores from dbName and create filename
	cleanDBName := strings.Trim(dbName, "_")
	filename := fmt.Sprintf("%s-%s.%s", cleanDBName, currentTime, extension)
	filepath := fmt.Sprintf("/tmp/%s", filename)

	// Run export command
	_, err := runner.RunCommand("sh", "-c", fmt.Sprintf("%s %s:export %s > %s", dokkuShellPrefix(), dbType, dbName, filepath))
	if err != nil {
		os.Remove(filepath)
		return "", fmt.Errorf("command execution error: %v", err)
	}

	// File check
	if info, err := os.Stat(filepath); err != nil || info.Size() == 0 {
		os.Remove(filepath)
		if err != nil {
			return "", fmt.Errorf("file check error: %v", err)
		}
		return "", fmt.Errorf("export file is empty")
	}

	return filename, nil
}
