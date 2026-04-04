package dokku

import "testing"

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
