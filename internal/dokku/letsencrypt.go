package dokku

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pruvon/pruvon/internal/models"
)

// GetSSLInfo returns SSL information for the specified app
func GetSSLInfo(runner CommandRunner, appName string) (models.SSLInfo, error) {
	output, err := runner.RunCommand("dokku", "letsencrypt:report", appName)
	if err != nil {
		return models.SSLInfo{}, fmt.Errorf("SSL information could not be retrieved: %v", err)
	}

	info := models.SSLInfo{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Letsencrypt active:") {
			info.Active = strings.TrimSpace(strings.Split(line, ":")[1]) == "true"
		} else if strings.Contains(line, "Letsencrypt autorenew:") {
			info.Autorenew = strings.TrimSpace(strings.Split(line, ":")[1]) == "true"
		} else if strings.Contains(line, "Letsencrypt computed email:") {
			info.Email = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.Contains(line, "Letsencrypt expiration:") {
			expStr := strings.TrimSpace(strings.Split(line, ":")[1])
			info.Expiration, _ = strconv.ParseInt(expStr, 10, 64)
		}
	}

	return info, nil
}

// IsLetsencryptInstalled checks if the letsencrypt plugin is installed
func IsLetsencryptInstalled(runner CommandRunner) (bool, error) {
	output, err := runner.RunCommand("dokku", "plugin:list")
	if err != nil {
		return false, fmt.Errorf("plugin list could not be retrieved: %v", err)
	}
	return strings.Contains(output, "letsencrypt"), nil
}
