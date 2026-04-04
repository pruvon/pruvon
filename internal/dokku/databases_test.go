package dokku

import (
	"errors"
	"strings"
	"testing"
)

func TestGetDatabases(t *testing.T) {
	tests := []struct {
		name          string
		dbType        string
		listOutput    string
		infoOutputs   map[string]string
		expectedDBs   int
		expectedError bool
		errorMsg      string
	}{
		{
			name:   "postgres databases",
			dbType: "postgres",
			listOutput: `testdb1
testdb2`,
			infoOutputs: map[string]string{
				"dokku postgres:info testdb1": "Status: running\nversion: 14.5",
				"dokku postgres:info testdb2": "Status: stopped\nversion: 13.0",
			},
			expectedDBs:   2,
			expectedError: false,
		},
		{
			name:   "mariadb databases",
			dbType: "mariadb",
			listOutput: `mydb
proddb`,
			infoOutputs: map[string]string{
				"dokku mariadb:info mydb":   "Status: running\nversion: 10.5",
				"dokku mariadb:info proddb": "Status: running\nversion: 10.6",
			},
			expectedDBs:   2,
			expectedError: false,
		},
		{
			name:   "redis databases",
			dbType: "redis",
			listOutput: `cache
sessions`,
			infoOutputs: map[string]string{
				"dokku redis:info cache":    "Status: running\nversion: 7.0",
				"dokku redis:info sessions": "Status: running\nversion: 6.2",
			},
			expectedDBs:   2,
			expectedError: false,
		},
		{
			name:   "mongo databases",
			dbType: "mongo",
			listOutput: `mongodb1
mongodb2`,
			infoOutputs: map[string]string{
				"dokku mongo:info mongodb1": "Status: running\nversion: 5.0",
				"dokku mongo:info mongodb2": "Status: stopped\nversion: 4.4",
			},
			expectedDBs:   2,
			expectedError: false,
		},
		{
			name:          "invalid database type",
			dbType:        "invalid",
			listOutput:    "",
			infoOutputs:   map[string]string{},
			expectedDBs:   0,
			expectedError: true,
			errorMsg:      "invalid database type",
		},
		{
			name:          "command error",
			dbType:        "postgres",
			listOutput:    "",
			infoOutputs:   map[string]string{},
			expectedDBs:   0,
			expectedError: true,
		},
		{
			name:   "empty database list",
			dbType: "postgres",
			listOutput: `=====> Databases
`,
			infoOutputs:   map[string]string{},
			expectedDBs:   0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &MockCommandRunner{
				OutputMap: map[string]string{},
				ErrorMap:  map[string]error{},
			}

			if tt.dbType != "invalid" && !tt.expectedError {
				mockRunner.OutputMap["dokku "+tt.dbType+":list"] = tt.listOutput
				for cmd, output := range tt.infoOutputs {
					mockRunner.OutputMap[cmd] = output
				}
			} else if tt.expectedError && tt.dbType != "invalid" {
				mockRunner.ErrorMap["dokku "+tt.dbType+":list"] = errors.New("command failed")
			}

			dbs, err := GetDatabases(mockRunner, tt.dbType)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(dbs) != tt.expectedDBs {
				t.Errorf("Expected %d databases, got %d", tt.expectedDBs, len(dbs))
			}
		})
	}
}
