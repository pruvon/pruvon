package dokku

import "testing"

func TestParseEnvVars(t *testing.T) {
	tests := []struct {
		name         string
		output       string
		expectedVars int
		expectedKeys []string
	}{
		{
			name: "multiple env vars",
			output: `=====> env vars
DATABASE_URL: postgres://user:pass@localhost/db
SECRET_KEY: mysecret123
PORT: 5000`,
			expectedVars: 3,
			expectedKeys: []string{"DATABASE_URL", "SECRET_KEY", "PORT"},
		},
		{
			name: "single env var",
			output: `=====> env vars
API_KEY: abc123`,
			expectedVars: 1,
			expectedKeys: []string{"API_KEY"},
		},
		{
			name: "no env vars",
			output: `=====> env vars
`,
			expectedVars: 0,
			expectedKeys: []string{},
		},
		{
			name:         "empty output",
			output:       "",
			expectedVars: 0,
			expectedKeys: []string{},
		},
		{
			name: "env vars with colon in value",
			output: `=====> env vars
URL: http://example.com:8080
TIME: 12:30:45`,
			expectedVars: 2,
			expectedKeys: []string{"URL", "TIME"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := ParseEnvVars(tt.output)

			if len(vars) != tt.expectedVars {
				t.Errorf("Expected %d env vars, got %d", tt.expectedVars, len(vars))
				return
			}

			for i, key := range tt.expectedKeys {
				if i < len(vars) && vars[i].Key != key {
					t.Errorf("Expected key[%d]=%s, got %s", i, key, vars[i].Key)
				}
			}
		})
	}
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedPorts int
	}{
		{
			name:          "single port mapping",
			output:        "Ports map: http:80:5000",
			expectedPorts: 1,
		},
		{
			name:          "multiple port mappings",
			output:        "Ports map: http:80:5000 https:443:5000",
			expectedPorts: 2,
		},
		{
			name:          "no port mappings",
			output:        "Ports map: ",
			expectedPorts: 0,
		},
		{
			name:          "empty output",
			output:        "",
			expectedPorts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := ParsePorts(tt.output)

			if len(ports) != tt.expectedPorts {
				t.Errorf("Expected %d port mappings, got %d", tt.expectedPorts, len(ports))
			}
		})
	}
}

func TestParseProcessInfo(t *testing.T) {
	tests := []struct {
		name              string
		output            string
		expectedProcesses int
	}{
		{
			name: "multiple processes",
			output: `Status web 1: running for 2h
Status worker 1: running for 2h
Status worker 2: running for 2h`,
			expectedProcesses: 2, // 2 process types (web, worker)
		},
		{
			name:              "single process",
			output:            `Status web 1: running for 1h`,
			expectedProcesses: 1,
		},
		{
			name: "with procfile path",
			output: `Ps computed procfile path: Procfile
Status web 1: running for 1h`,
			expectedProcesses: 2, // ProcfilePath + web
		},
		{
			name:              "empty output",
			output:            "",
			expectedProcesses: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processes := ParseProcessInfo(tt.output)

			if len(processes) != tt.expectedProcesses {
				t.Errorf("Expected %d processes, got %d", tt.expectedProcesses, len(processes))
			}
		})
	}
}

func TestParseNginxConfig(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedConfig int
	}{
		{
			name: "both config values",
			output: `Nginx computed client max body size: 1m
Nginx computed proxy read timeout: 60s`,
			expectedConfig: 2,
		},
		{
			name:           "client max body size only",
			output:         "Nginx computed client max body size: 10m",
			expectedConfig: 1,
		},
		{
			name:           "proxy read timeout only",
			output:         "Nginx computed proxy read timeout: 120s",
			expectedConfig: 1,
		},
		{
			name:           "no config values",
			output:         "Some other output",
			expectedConfig: 0,
		},
		{
			name:           "empty output",
			output:         "",
			expectedConfig: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs := ParseNginxConfig(tt.output)

			if len(configs) != tt.expectedConfig {
				t.Errorf("Expected %d config items, got %d", tt.expectedConfig, len(configs))
			}
		})
	}
}
