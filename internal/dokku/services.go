package dokku

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pruvon/pruvon/internal/models"
)

// parseServiceVersion extracts version information from service info output
func parseServiceVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "version") || strings.Contains(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// parseServiceStatus parses the output of the dokku <service>:list <name> command
// to determine if the service is running
func parseServiceStatus(output string) string {
	// Default status is "stopped"
	status := "stopped"

	// Check if the output contains status information
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for status, running, or container information
		if strings.Contains(line, "Status:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				status = strings.TrimSpace(parts[1])
				// If status is explicitly "running", return it
				if status == "running" {
					return status
				}
			}
		} else if strings.Contains(line, "Container") && !strings.Contains(line, "not exists") {
			// If a container is mentioned and it's not "not exists", the service is likely running
			if !strings.Contains(line, "not exists") && !strings.Contains(line, "stopped") {
				return "running"
			}
		}
	}

	return status
}

// GetLinkedApps returns a list of application names linked to a specific database service
func GetLinkedApps(runner CommandRunner, serviceType string, serviceName string) ([]string, error) {
	// First try the dedicated links command which prints linked apps line-by-line
	if linksOutput, err := runner.RunCommand("dokku", fmt.Sprintf("%s:links", serviceType), serviceName); err == nil {
		var apps []string
		for _, line := range strings.Split(linksOutput, "\n") {
			l := strings.TrimSpace(line)
			if l == "" {
				continue
			}
			// Skip headers/separators
			if strings.HasPrefix(l, "===") || strings.HasPrefix(l, "=====") {
				continue
			}
			// Some outputs may include a label line, skip if it looks like a label
			lower := strings.ToLower(l)
			if strings.Contains(lower, "links") && strings.Contains(lower, "database") {
				continue
			}
			// Handle possible bullet format like "- app-name" or plain app name
			l = strings.TrimPrefix(l, "- ")
			// If the line has multiple fields, the app name is the first token
			fields := strings.Fields(l)
			if len(fields) > 0 {
				apps = append(apps, fields[0])
			}
		}
		if len(apps) > 0 {
			return apps, nil
		}
		// If no apps parsed, fall through to info parsing as a fallback
	}

	// Fallback: parse from the info output
	infoCmd := fmt.Sprintf("%s:info", serviceType)
	output, err := runner.RunCommand("dokku", infoCmd, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service info: %v", err)
	}

	var linkedApps []string
	lines := strings.Split(output, "\n")

	// Try inline format first: "Links: app1 app2" or "Linked apps: app1 app2"
	startIndex := -1
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "linked apps:") || strings.Contains(lower, "links:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if value != "-" && value != "" {
					linkedApps = strings.Fields(value)
					if len(linkedApps) > 0 {
						return linkedApps, nil
					}
				}
			}
			// If nothing on the same line, record start to read following lines
			startIndex = i + 1
			break
		}
	}

	// If we found a label but no inline values, read subsequent lines until next section
	if startIndex != -1 {
		for j := startIndex; j < len(lines); j++ {
			l := strings.TrimSpace(lines[j])
			if l == "" {
				continue
			}
			if strings.HasPrefix(l, "===") || strings.Contains(l, ":") {
				// Likely reached a new section or header
				break
			}
			l = strings.TrimPrefix(l, "- ")
			fields := strings.Fields(l)
			if len(fields) > 0 {
				linkedApps = append(linkedApps, fields[0])
			}
		}
	}

	return linkedApps, nil
}

// GetServices returns a list of services of the specified type
func GetServices(runner CommandRunner, svcType string) ([]models.Service, error) {
	if runner == nil {
		log.Printf("WARNING: CommandRunner is nil in GetServices, using DefaultCommandRunner")
		runner = DefaultCommandRunner
	}

	// Check if the service type is in our supported database plugins list
	dbPlugins := GetDatabasePluginList()
	isValidDbPlugin := false
	for _, plugin := range dbPlugins {
		if plugin == svcType {
			isValidDbPlugin = true
			break
		}
	}

	// If not a valid database plugin, check if it's in our service plugins list
	if !isValidDbPlugin {
		servicePlugins := GetServicePluginList()
		isValidServicePlugin := false
		for _, plugin := range servicePlugins {
			if plugin == svcType {
				isValidServicePlugin = true
				break
			}
		}

		// If not found in either list, return an error
		if !isValidServicePlugin {
			return nil, fmt.Errorf("invalid service type: %s", svcType)
		}
	}

	// Check if the plugin is installed
	installedPlugins, err := GetInstalledPluginsMap(runner)
	if err != nil {
		return nil, fmt.Errorf("plugin durumu kontrol edilemedi: %v", err)
	}

	if !installedPlugins[svcType] {
		log.Printf("Plugin %s is not installed", svcType)
		return []models.Service{}, nil
	}

	cmd := svcType + ":list"

	output, err := runner.RunCommand("dokku", cmd)
	if err != nil {
		log.Printf("Error getting services for type %s: %v", svcType, err)
		// Try with a direct command as fallback
		directOutput, directErr := exec.Command("dokku", cmd).Output()
		if directErr != nil {
			log.Printf("Direct command also failed: %v", directErr)
			return nil, fmt.Errorf("service list could not be retrieved: %v (dokku plugin may not be installed)", err)
		}
		output = string(directOutput)
	}

	lines := strings.Split(output, "\n")
	var svcs []models.Service
	for _, line := range lines {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		name := strings.TrimSpace(line)
		svcs = append(svcs, models.Service{
			Name: name,
		})
	}

	// Get status information for each service
	_, err = runner.RunCommand("dokku", svcType+":list")
	if err == nil {
		// Parse the detailed status output
		for i := range svcs {
			// Run info command for each individual service to get detailed status
			svcInfoOutput, err := runner.RunCommand("dokku", svcType+":info", svcs[i].Name)
			if err == nil {
				// Find status of this specific service
				status := parseServiceStatus(svcInfoOutput)
				svcs[i].Status = status

				// Also try to extract version information
				version := parseServiceVersion(svcInfoOutput)
				if version != "" {
					svcs[i].Version = version
				}
			}
		}
	}

	// If we couldn't find any services but didn't error, return an empty array
	if len(svcs) == 0 {
		log.Printf("No services found for type %s", svcType)
		return []models.Service{}, nil
	}

	return svcs, nil
}

// GetServiceInfo returns detailed information about a service
func GetServiceInfo(runner CommandRunner, svcType string, svcName string) (models.ServiceInstanceInfo, error) {
	info := models.ServiceInstanceInfo{
		Name:         svcName,
		Service:      svcType,
		InstanceName: svcName,
		Details:      make(map[string]string),
	}

	// Get database plugin list
	dbPlugins := GetDatabasePluginList()
	isDatabase := false

	// Check if service type is a database
	for _, dbType := range dbPlugins {
		if svcType == dbType {
			isDatabase = true
			break
		}
	}

	// Set type based on whether it's a database service
	if isDatabase {
		info.Type = "database"
	} else {
		info.Type = svcType
	}

	var output string
	var err error

	// Run the appropriate info command based on service type
	output, err = runner.RunCommand("dokku", svcType+":info", svcName)
	if err != nil {
		return info, fmt.Errorf("service information could not be retrieved: %v", err)
	}

	lines := strings.Split(output, "\n")
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

	// Get linked applications
	linkedApps, err := GetLinkedApps(runner, svcType, svcName)
	if err == nil {
		info.LinkInfo = &models.LinkInfo{
			LinkedApps: linkedApps,
		}
	}

	return info, nil
}

// ExportService exports a service to a temporary file and returns the filename
func ExportService(runner CommandRunner, svcType string, svcName string) (string, error) {
	// Create a temporary filename for the original export
	timestamp := time.Now().Unix()
	// Format timestamp in YEAR-MONTH-DAY-HOUR format
	timeFormatted := time.Unix(timestamp, 0).Format("2006-01-02-1504")
	tempBasename := fmt.Sprintf("%s_%s_%s.dump", svcType, svcName, timeFormatted)
	origTempPath := filepath.Join("/tmp", tempBasename)

	// Final gz filename
	finalFilename := fmt.Sprintf("%s.gz", tempBasename)
	finalPath := filepath.Join("/tmp", finalFilename)

	// The command differs based on service type
	var cmd string
	switch svcType {
	case "postgres", "mariadb", "mongo", "redis":
		// Use shell redirection to export the service data to a file
		cmd = fmt.Sprintf("dokku %s:export %s > %s", svcType, svcName, origTempPath)
		_, err := runner.RunCommand("sh", "-c", cmd)
		if err != nil {
			return "", fmt.Errorf("service could not be exported: %v", err)
		}
	default:
		return "", fmt.Errorf("unsupported service type: %s", svcType)
	}

	// Verify the file exists and has content
	fileInfo, err := os.Stat(origTempPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("export file could not be created: %v", err)
		}
		return "", fmt.Errorf("file information could not be retrieved: %v", err)
	}

	// Check if file is empty
	if fileInfo.Size() == 0 {
		os.Remove(origTempPath)
		return "", fmt.Errorf("export file was created empty")
	}

	// Create gz file from the original file
	gzCmd := fmt.Sprintf("gzip -c %s > %s", origTempPath, finalPath)

	_, err = runner.RunCommand("sh", "-c", gzCmd)
	if err != nil {
		// Clean up original file
		os.Remove(origTempPath)
		return "", fmt.Errorf("file could not be compressed: %v", err)
	}

	// Remove the original uncompressed file
	os.Remove(origTempPath)

	// Verify the gz file exists and has content
	gzInfo, err := os.Stat(finalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("gz file could not be created: %v", err)
		}
		return "", fmt.Errorf("gz file information could not be retrieved: %v", err)
	}

	// Check if gz file is empty
	if gzInfo.Size() == 0 {
		os.Remove(finalPath)
		return "", fmt.Errorf("gz dosyası boş olarak oluşturuldu")
	}

	// Ensure file has proper permissions for reading
	if err := os.Chmod(finalPath, 0644); err != nil {
		return "", fmt.Errorf("dosya izinleri ayarlanamadı: %v", err)
	}

	// Return the name of the gz file
	return finalFilename, nil
}

// GetServiceBasicInfo returns only essential information about a service for quick loading
func GetServiceBasicInfo(runner CommandRunner, svcType string, svcName string) (models.ServiceInstanceInfo, error) {
	info := models.ServiceInstanceInfo{
		Name:         svcName,
		Service:      svcType,
		InstanceName: svcName,
		Details:      make(map[string]string),
	}

	// Get database plugin list
	dbPlugins := GetDatabasePluginList()
	isDatabase := false

	// Check if service type is a database
	for _, dbType := range dbPlugins {
		if svcType == dbType {
			isDatabase = true
			break
		}
	}

	// Set type based on whether it's a database service
	if isDatabase {
		info.Type = "database"
	} else {
		info.Type = svcType
	}

	var output string
	var err error

	// Run the appropriate info command based on service type
	output, err = runner.RunCommand("dokku", svcType+":info", svcName)
	if err != nil {
		return info, fmt.Errorf("servis bilgisi alınamadı: %v", err)
	}

	lines := strings.Split(output, "\n")
	var url string
	var version string
	var status string
	var internalIP string

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

		// Extract only the basic information we need
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
		}
	}

	// Update the struct with the extracted information
	info.URL = url
	info.Version = version
	info.Status = status

	// Add internal IP to the details map if available
	if internalIP != "" {
		if info.Details == nil {
			info.Details = make(map[string]string)
		}
		info.Details["Internal IP"] = internalIP
	}

	return info, nil
}

// GetServiceResourceInfo returns only the resource limits for a service
func GetServiceResourceInfo(runner CommandRunner, svcType string, svcName string) (*models.ResourceLimits, error) {
	// First we need to get the container ID
	containerID, err := getServiceContainerId(runner, svcType, svcName)
	if err != nil {
		return nil, err
	}

	// If we don't have a container ID, there are no resources to check
	if containerID == "" {
		return nil, nil
	}

	// Initialize resource limits struct
	resourceLimits := &models.ResourceLimits{}

	// Fetch resource limits from Docker inspect
	inspectOutput, err := runner.RunCommand("docker", "inspect", containerID)
	if err != nil {
		return nil, fmt.Errorf("docker inspect failed: %v", err)
	}

	// Parse the JSON output from docker inspect
	var containerDetails []map[string]interface{}
	if err := json.Unmarshal([]byte(inspectOutput), &containerDetails); err != nil {
		return nil, fmt.Errorf("failed to parse docker inspect output: %v", err)
	}

	if len(containerDetails) == 0 {
		return nil, fmt.Errorf("no container details found")
	}

	// Extract resource limits
	if hostConfig, ok := containerDetails[0]["HostConfig"].(map[string]interface{}); ok {
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
	}

	return resourceLimits, nil
}

// getServiceContainerId retrieves just the container ID for a service
func getServiceContainerId(runner CommandRunner, svcType string, svcName string) (string, error) {
	output, err := runner.RunCommand("dokku", svcType+":info", svcName)
	if err != nil {
		return "", fmt.Errorf("servis bilgisi alınamadı: %v", err)
	}

	lines := strings.Split(output, "\n")

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

		// Look for container ID
		if strings.Contains(strings.ToLower(key), "container") && !strings.Contains(strings.ToLower(key), "mount") {
			// Get container ID - usually in format "Container: a72f95864f43" or similar
			// Make sure this is not a container mount path
			if strings.Contains(value, " ") {
				parts := strings.Fields(value)
				if len(parts) > 0 {
					return parts[0], nil
				}
			} else {
				return value, nil
			}
		} else if key == "Id" {
			// Get container ID from the "Id" field
			return value, nil
		}
	}

	return "", nil
}

// GetServiceNamesOnly returns just the names of services, without fetching status info
// This is a more lightweight call than GetServices when only names are needed
func GetServiceNamesOnly(runner CommandRunner, svcType string) ([]string, error) {
	if runner == nil {
		log.Printf("WARNING: CommandRunner is nil in GetServiceNamesOnly, using DefaultCommandRunner")
		runner = DefaultCommandRunner
	}

	// Verify service type is valid
	dbPlugins := GetDatabasePluginList()
	servicePlugins := GetServicePluginList()

	isValidType := false
	// Check in database plugins
	for _, plugin := range dbPlugins {
		if plugin == svcType {
			isValidType = true
			break
		}
	}

	// If not found in database plugins, check in service plugins
	if !isValidType {
		for _, plugin := range servicePlugins {
			if plugin == svcType {
				isValidType = true
				break
			}
		}
	}

	// Return error if service type is not valid
	if !isValidType {
		return nil, fmt.Errorf("geçersiz servis tipi: %s", svcType)
	}

	// Check if the plugin is installed
	installedPlugins, err := GetInstalledPluginsMap(runner)
	if err != nil {
		return nil, fmt.Errorf("plugin durumu kontrol edilemedi: %v", err)
	}

	if !installedPlugins[svcType] {
		log.Printf("Plugin %s is not installed", svcType)
		return []string{}, nil
	}

	// Run the list command to get service names
	cmd := svcType + ":list"
	output, err := runner.RunCommand("dokku", cmd)
	if err != nil {
		log.Printf("Error getting services for type %s: %v", svcType, err)
		return nil, fmt.Errorf("servis listesi alınamadı: %v", err)
	}

	// Parse the output to extract service names
	lines := strings.Split(output, "\n")
	var serviceNames []string
	for _, line := range lines {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		name := strings.TrimSpace(line)
		serviceNames = append(serviceNames, name)
	}

	return serviceNames, nil
}

// GetInstalledServicePluginsByFilesystem checks the filesystem to detect installed service plugins.
// This is much faster than running "dokku plugin:list" command.
// Returns a list of installed service plugin names.
func GetInstalledServicePluginsByFilesystem() ([]string, error) {
	const servicesBaseDir = "/var/lib/dokku/services"

	// List of all possible service types we support
	allServiceTypes := []string{
		"postgres", "mariadb", "mongo", "redis", "rabbitmq", "memcached",
		"clickhouse", "elasticsearch", "nats", "solr", "rethinkdb",
		"couchdb", "meilisearch", "pushpin", "omnisci",
	}

	// Initialize with empty slice to ensure it's never nil
	installedServices := []string{}

	// Check if the base services directory exists
	if _, err := os.Stat(servicesBaseDir); os.IsNotExist(err) {
		// Directory doesn't exist, no services installed
		log.Printf("Services directory %s does not exist, no services installed", servicesBaseDir)
		return installedServices, nil
	}

	// Check each service type directory
	for _, svcType := range allServiceTypes {
		svcDir := filepath.Join(servicesBaseDir, svcType)

		// Check if this service type directory exists
		if info, err := os.Stat(svcDir); err == nil && info.IsDir() {
			installedServices = append(installedServices, svcType)
		}
	}

	return installedServices, nil
}

// GetServiceNamesByFilesystem lists service instances by checking the filesystem.
// This is much faster than running "dokku {type}:list" command.
// Returns a list of service instance names for the given service type.
func GetServiceNamesByFilesystem(svcType string) ([]string, error) {
	const servicesBaseDir = "/var/lib/dokku/services"

	svcDir := filepath.Join(servicesBaseDir, svcType)

	// Check if the service type directory exists
	if _, err := os.Stat(svcDir); os.IsNotExist(err) {
		// Directory doesn't exist, this service type is not installed
		log.Printf("Service directory %s does not exist", svcDir)
		return []string{}, nil
	}

	// Read the directory entries
	entries, err := os.ReadDir(svcDir)
	if err != nil {
		log.Printf("Error reading service directory %s: %v", svcDir, err)
		return nil, fmt.Errorf("failed to read service directory: %v", err)
	}

	var serviceNames []string
	for _, entry := range entries {
		// Only include directories (each service instance has its own directory)
		if entry.IsDir() {
			serviceNames = append(serviceNames, entry.Name())
		}
	}

	return serviceNames, nil
}
