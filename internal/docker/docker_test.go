package docker

import (
	"errors"
	"os"
	"testing"
)

// MockCommandRunner for testing
type MockCommandRunner struct {
	OutputMap map[string]string
	ErrorMap  map[string]error
}

func (m *MockCommandRunner) RunCommand(command string, args ...string) (string, error) {
	// Build the full command string for lookup
	fullCmd := command
	for _, arg := range args {
		fullCmd += " " + arg
	}

	if err, exists := m.ErrorMap[fullCmd]; exists {
		return "", err
	}

	if output, exists := m.OutputMap[fullCmd]; exists {
		return output, nil
	}

	return "", nil
}

func (m *MockCommandRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, errors.New("not implemented")
}

func TestGetContainerStats(t *testing.T) {
	tests := []struct {
		name               string
		appName            string
		containersOutput   string
		statsOutput        string
		containersError    error
		statsError         error
		expectedIsDeployed bool
		expectedCPU        float64
		expectedMemory     float64
	}{
		{
			name:               "deployed app with stats",
			appName:            "myapp",
			containersOutput:   "abc123\tdokku/myapp:latest\t/start web\tUp 2 hours\t0.0.0.0:80->5000/tcp\tmyapp.web.1",
			statsOutput:        "25.50%\t45.30%",
			expectedIsDeployed: true,
			expectedCPU:        25.50,
			expectedMemory:     45.30,
		},
		{
			name:               "not deployed app",
			appName:            "notdeployed",
			containersOutput:   "",
			expectedIsDeployed: false,
			expectedCPU:        0,
			expectedMemory:     0,
		},
		{
			name:               "deployed but stats error",
			appName:            "errorapp",
			containersOutput:   "def456\tdokku/errorapp:latest\t/start web\tUp 1 hour\t\terrorapp.web.1",
			statsError:         errors.New("stats error"),
			expectedIsDeployed: true,
			expectedCPU:        0,
			expectedMemory:     0,
		},
		{
			name:               "containers error",
			appName:            "errorapp",
			containersError:    errors.New("docker error"),
			expectedIsDeployed: false,
			expectedCPU:        0,
			expectedMemory:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &MockCommandRunner{
				OutputMap: map[string]string{},
				ErrorMap:  map[string]error{},
			}

			// getAppContainers uses "docker ps --format ..." without filter
			containerCmd := "docker ps --format {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}"

			if tt.containersError != nil {
				mockRunner.ErrorMap[containerCmd] = tt.containersError
			} else {
				mockRunner.OutputMap[containerCmd] = tt.containersOutput
			}

			if tt.statsError != nil {
				statsCmd := "docker stats --no-stream --format {{.CPUPerc}}\t{{.MemPerc}} abc123"
				mockRunner.ErrorMap[statsCmd] = tt.statsError
				statsCmd2 := "docker stats --no-stream --format {{.CPUPerc}}\t{{.MemPerc}} def456"
				mockRunner.ErrorMap[statsCmd2] = tt.statsError
			} else if tt.statsOutput != "" {
				statsCmd := "docker stats --no-stream --format {{.CPUPerc}}\t{{.MemPerc}} abc123"
				mockRunner.OutputMap[statsCmd] = tt.statsOutput
				statsCmd2 := "docker stats --no-stream --format {{.CPUPerc}}\t{{.MemPerc}} def456"
				mockRunner.OutputMap[statsCmd2] = tt.statsOutput
			}

			stats := GetContainerStats(mockRunner, tt.appName)

			if stats.IsDeployed != tt.expectedIsDeployed {
				t.Errorf("Expected IsDeployed=%v, got %v", tt.expectedIsDeployed, stats.IsDeployed)
			}

			if stats.CPUUsage != tt.expectedCPU {
				t.Errorf("Expected CPUUsage=%.2f, got %.2f", tt.expectedCPU, stats.CPUUsage)
			}

			if stats.MemoryUsage != tt.expectedMemory {
				t.Errorf("Expected MemoryUsage=%.2f, got %.2f", tt.expectedMemory, stats.MemoryUsage)
			}
		})
	}
}
