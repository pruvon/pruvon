package dokku

import (
	"strings"
)

// GetDokkuVersion returns the installed Dokku version
func GetDokkuVersion(runner CommandRunner) (string, error) {
	output, err := runner.RunCommand("dokku", "--version")
	if err != nil {
		return "", err
	}
	// Extract only the version number from "dokku version 0.34.4" format
	parts := strings.Fields(output)
	if len(parts) >= 3 {
		return parts[2], nil // Last part is the version number
	}
	return strings.TrimSpace(output), nil // Fallback: return all output
}
