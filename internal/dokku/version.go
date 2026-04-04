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
	// "dokku version 0.34.4" formatından sadece sürüm numarasını al
	parts := strings.Fields(output)
	if len(parts) >= 3 {
		return parts[2], nil // Son parça sürüm numarası
	}
	return strings.TrimSpace(output), nil // Fallback olarak tüm çıktıyı döndür
}
