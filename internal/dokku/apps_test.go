package dokku

import (
	"errors"
	"testing"
)

func TestGetDokkuApps(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedApps  []string
		expectedError bool
	}{
		{
			name: "multiple apps",
			output: `=====> My Apps
app1
app2
app3`,
			expectedApps:  []string{"app1", "app2", "app3"},
			expectedError: false,
		},
		{
			name:          "no apps",
			output:        "=====> My Apps",
			expectedApps:  []string{},
			expectedError: false,
		},
		{
			name: "apps with extra whitespace",
			output: `=====> My Apps
app1
app2
app3`,
			expectedApps:  []string{"app1", "app2", "app3"},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &MockCommandRunner{
				OutputMap: map[string]string{
					"dokku apps:list": tt.output,
				},
				ErrorMap: map[string]error{},
			}

			apps, err := GetDokkuApps(mockRunner)

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(apps) != len(tt.expectedApps) {
				t.Errorf("Expected %d apps, got %d", len(tt.expectedApps), len(apps))
			}

			for i, app := range apps {
				if i < len(tt.expectedApps) && app != tt.expectedApps[i] {
					t.Errorf("Expected app[%d]=%s, got %s", i, tt.expectedApps[i], app)
				}
			}
		})
	}
}

func TestGetDokkuApps_Error(t *testing.T) {
	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{},
		ErrorMap: map[string]error{
			"dokku apps:list": errors.New("command failed"),
		},
	}

	_, err := GetDokkuApps(mockRunner)
	if err == nil {
		t.Error("Expected error when command fails")
	}
}

func TestGetAppContainers(t *testing.T) {
	mockOutput := `abc123def456	dokku/myapp:latest	/start web	Up 2 hours	0.0.0.0:80->5000/tcp	myapp.web.1
def456ghi789	dokku/myapp:latest	/start worker	Up 2 hours		myapp.worker.1`

	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{
			"docker ps --format {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}": mockOutput,
		},
		ErrorMap: map[string]error{},
	}

	containers, err := GetAppContainers(mockRunner, "myapp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}

	// Check first container
	if containers[0].ID != "abc123def456" {
		t.Errorf("Expected container ID 'abc123def456', got '%s'", containers[0].ID)
	}
	if containers[0].Name != "myapp.web.1" {
		t.Errorf("Expected container name 'myapp.web.1', got '%s'", containers[0].Name)
	}
	if containers[0].Status != "Up 2 hours" {
		t.Errorf("Expected container status 'Up 2 hours', got '%s'", containers[0].Status)
	}
}

func TestGetAppContainers_NoContainers(t *testing.T) {
	mockOutput := ``

	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{
			"docker ps --format {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}": mockOutput,
		},
		ErrorMap: map[string]error{},
	}

	containers, err := GetAppContainers(mockRunner, "testapp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(containers))
	}
}

func TestGetAppContainers_Error(t *testing.T) {
	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{},
		ErrorMap: map[string]error{
			"docker ps --format {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}": errors.New("docker daemon not running"),
		},
	}

	_, err := GetAppContainers(mockRunner, "nonexistent")
	if err == nil {
		t.Error("Expected error when docker command fails")
	}
}
