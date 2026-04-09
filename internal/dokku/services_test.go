package dokku

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseServiceVersion(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "version with version field",
			output:   "version: 14.5\nSomething: else",
			expected: "14.5",
		},
		{
			name:     "Version with capital V",
			output:   "Version: 5.7.30\nStatus: running",
			expected: "5.7.30",
		},
		{
			name:     "version without version field",
			output:   "Status: running\nPort: 5432",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseServiceVersion(tt.output)
			if result != tt.expected {
				t.Errorf("Expected version '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseServiceStatus(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "running status",
			output:   "Status: running\nImage: postgres:14",
			expected: "running",
		},
		{
			name:     "explicitly stopped status",
			output:   "Status: stopped\nImage: redis:7",
			expected: "stopped",
		},
		{
			name:     "no status field defaults to stopped",
			output:   "Image: mysql:8.0\nPort: 3306",
			expected: "stopped",
		},
		{
			name:     "empty output defaults to stopped",
			output:   "",
			expected: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseServiceStatus(tt.output)
			if result != tt.expected {
				t.Errorf("Expected status '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

type exportMockRunner struct {
	commands []string
}

func (m *exportMockRunner) RunCommand(command string, args ...string) (string, error) {
	if command != "sh" || len(args) != 2 || args[0] != "-c" {
		return "", fmt.Errorf("unexpected command: %s %v", command, args)
	}

	script := args[1]
	m.commands = append(m.commands, script)

	parts := strings.SplitN(script, " > ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected shell script: %s", script)
	}

	target := strings.TrimSpace(parts[1])

	if strings.Contains(parts[0], ":export ") {
		return "", os.WriteFile(target, []byte("export-data"), 0644)
	}

	if strings.HasPrefix(parts[0], "gzip -c ") {
		return "", os.WriteFile(target, []byte("compressed-data"), 0644)
	}

	return "", nil
}

func (m *exportMockRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, nil
}

func TestExportServiceUsesDokkuShellPrefix(t *testing.T) {
	runner := &exportMockRunner{}
	serviceName := fmt.Sprintf("service-%d", time.Now().UnixNano())

	filename, err := ExportService(runner, "postgres", serviceName)
	if err != nil {
		t.Fatalf("ExportService returned error: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Remove(filepath.Join("/tmp", filename))
	})

	if len(runner.commands) == 0 {
		t.Fatal("expected export command to be executed")
	}

	wantPrefix := "dokku postgres:export "
	if os.Geteuid() != 0 {
		wantPrefix = "sudo -n dokku postgres:export "
	}

	if !strings.HasPrefix(runner.commands[0], wantPrefix) {
		t.Fatalf("expected export command to start with %q, got %q", wantPrefix, runner.commands[0])
	}
}

func TestExportServiceSkipsSudoWhenDisabled(t *testing.T) {
	t.Setenv("PRUVON_DISABLE_SUDO", "1")

	runner := &exportMockRunner{}
	serviceName := fmt.Sprintf("service-%d", time.Now().UnixNano())

	filename, err := ExportService(runner, "postgres", serviceName)
	if err != nil {
		t.Fatalf("ExportService returned error: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Remove(filepath.Join("/tmp", filename))
	})

	if len(runner.commands) == 0 {
		t.Fatal("expected export command to be executed")
	}

	if !strings.HasPrefix(runner.commands[0], "dokku postgres:export ") {
		t.Fatalf("expected sudo-free export command, got %q", runner.commands[0])
	}
}
