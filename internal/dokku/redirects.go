package dokku

import (
	"fmt"
	"strings"

	"github.com/pruvon/pruvon/internal/models"
)

// IsRedirectPluginInstalled checks if the redirect plugin is installed
func IsRedirectPluginInstalled(runner CommandRunner) (bool, error) {
	output, err := runner.RunCommand("dokku", "plugin:list")
	if err != nil {
		return false, fmt.Errorf("plugin list could not be retrieved: %v", err)
	}
	return strings.Contains(output, "redirect"), nil
}

// GetRedirects returns a list of redirects for the specified app
func GetRedirects(runner CommandRunner, appName string) ([]models.Redirect, error) {
	output, err := runner.RunCommand("dokku", "redirect", appName)
	if err != nil {
		return nil, fmt.Errorf("redirects could not be retrieved: %v", err)
	}

	var redirects []models.Redirect
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "SOURCE") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			redirects = append(redirects, models.Redirect{
				Source:      fields[0],
				Destination: fields[1],
				Code:        fields[2],
			})
		}
	}
	return redirects, nil
}
