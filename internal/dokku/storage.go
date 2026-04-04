package dokku

import (
	"fmt"
	"strings"

	"github.com/pruvon/pruvon/internal/models"
)

// ParseStorageInfo extracts storage information for the specified app
func ParseStorageInfo(runner CommandRunner, appName string) (models.StorageInfo, error) {
	info := models.StorageInfo{}

	// Get storage mounts
	output, err := runner.RunCommand("dokku", "storage:list", appName)
	if err != nil {
		return info, fmt.Errorf("storage information could not be retrieved: %v", err)
	}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		source := parts[0]
		destination := parts[1]

		// Get size of the mount point
		size := "N/A"
		duOutput, err := runner.RunCommand("du", "-sh", source)
		if err == nil {
			if duParts := strings.Fields(duOutput); len(duParts) > 0 {
				size = duParts[0]
			}
		}

		info.Mounts = append(info.Mounts, models.StorageMount{
			Source:      source,
			Destination: destination,
			Size:        size,
		})
	}

	// Calculate total disk usage
	totalUsage, err := runner.RunCommand("du", "-sh", "/var/lib/dokku/data/storage/"+appName)
	if err == nil {
		if parts := strings.Fields(totalUsage); len(parts) > 0 {
			info.DiskUsage = parts[0]
		}
	}

	return info, nil
}
