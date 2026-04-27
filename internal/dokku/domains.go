package dokku

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ParseDomains extracts domain information from Dokku output
func ParseDomains(output string) []string {
	var domains []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Domains app vhosts:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				domainList := strings.TrimSpace(parts[1])
				if domainList != "" {
					domains = strings.Fields(domainList)
				}
			}
		}
	}
	return domains
}

// GetDomains returns a list of domains configured for an app
func GetDomains(runner CommandRunner, appName string) ([]string, error) {
	output, err := runner.RunCommand("dokku", "domains:report", appName)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	var domains []string

	for _, line := range lines {
		if strings.Contains(line, "Domains global") || strings.Contains(line, "Domains app-specific") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				domainList := strings.TrimSpace(parts[1])
				if domainList != "" {
					for _, domain := range strings.Split(domainList, " ") {
						if domain != "" {
							domains = append(domains, domain)
						}
					}
				}
			}
		}
	}

	return domains, nil
}

// ReadVHostFile reads the content of the VHOST file
func ReadVHostFile() (string, error) {
	vhostPath := "/home/dokku/VHOST"

	// For testing/development, use /tmp/VHOST if the real file doesn't exist
	if _, err := os.Stat(vhostPath); os.IsNotExist(err) {
		tmpPath := "/tmp/VHOST"
		if _, err := os.Stat(tmpPath); err == nil {
			vhostPath = tmpPath
		}
	}

	content, err := os.ReadFile(vhostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read VHOST file: %v", err)
	}

	return strings.TrimSpace(string(content)), nil
}

// WriteVHostFile writes content to the VHOST file
func WriteVHostFile(content string) error {
	vhostPath := "/home/dokku/VHOST"

	// For testing/development, use /tmp/VHOST if the real file doesn't exist
	if _, err := os.Stat(vhostPath); os.IsNotExist(err) {
		tmpPath := "/tmp/VHOST"
		// Create the tmp file if it doesn't exist yet
		if _, err := os.Stat(tmpPath); err == nil || os.IsNotExist(err) {
			vhostPath = tmpPath
		}
	}

	// Ensure the content ends with a newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	err := os.WriteFile(vhostPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write VHOST file: %v", err)
	}

	return nil
}

// GetServerDomain returns the server domain
func GetServerDomain(runner CommandRunner) (string, error) {
	// First check from environment variable
	domain := os.Getenv("DOKKU_DOMAIN")
	if domain != "" {
		return domain, nil
	}

	// If no environment variable, get the dokku global domain
	output, err := runner.RunCommand("dokku", "domains:report", "--global")
	if err != nil {
		return "", fmt.Errorf("global domain information could not be retrieved: %v", err)
	}

	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Domains global vhosts:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				domains := strings.Fields(strings.TrimSpace(parts[1]))
				if len(domains) > 0 {
					return domains[0], nil
				}
			}
		}
	}

	// If nothing is found, return the IP address
	resp, err := http.Get("https://api.ipify.org")
	if err == nil {
		defer resp.Body.Close()
		if ip, err := io.ReadAll(resp.Body); err == nil {
			return string(ip), nil
		}
	}
	return "", nil
}
