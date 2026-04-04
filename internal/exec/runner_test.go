package exec

import (
	"testing"
)

func TestRealCommandRunner_RunCommand(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name          string
		command       string
		args          []string
		expectError   bool
		expectOutput  string
		expectPartial bool
	}{
		{
			name:          "Valid Command",
			command:       "echo",
			args:          []string{"test"},
			expectError:   false,
			expectOutput:  "test\n",
			expectPartial: false,
		},
		{
			name:          "Non-existent Command",
			command:       "nonexistentcommand",
			args:          []string{},
			expectError:   true,
			expectOutput:  "",
			expectPartial: false,
		},
		{
			name:          "Partial Output Check",
			command:       "ls",
			args:          []string{"-la"},
			expectError:   false,
			expectOutput:  "total",
			expectPartial: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create CommandRunner
			runner := NewCommandRunner()

			// Run command
			output, err := runner.RunCommand(tc.command, tc.args...)

			// Check error
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Did not expect error but got: %v", err)
			}

			// Check output
			if !tc.expectError {
				if tc.expectPartial {
					// Check partial match
					if len(output) < len(tc.expectOutput) || output[:len(tc.expectOutput)] != tc.expectOutput {
						t.Errorf("Output should start with '%s', got: '%s'", tc.expectOutput, output)
					}
				} else if output != tc.expectOutput {
					t.Errorf("Expected output: '%s', got: '%s'", tc.expectOutput, output)
				}
			}
		})
	}
}

func TestRealCommandRunner_StartPTY(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name        string
		command     string
		args        []string
		expectError bool
	}{
		{
			name:        "Valid Command",
			command:     "echo",
			args:        []string{"test"},
			expectError: false,
		},
		{
			name:        "Non-existent Command",
			command:     "nonexistentcommand",
			args:        []string{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create CommandRunner
			runner := NewCommandRunner()

			// Run command
			ptyFile, err := runner.StartPTY(tc.command, tc.args...)

			// Check error
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
				if ptyFile != nil {
					ptyFile.Close()
				}
			}
			if !tc.expectError && err != nil {
				t.Errorf("Did not expect error but got: %v", err)
			}

			// Close PTY file descriptor on success
			if ptyFile != nil {
				if err := ptyFile.Close(); err != nil {
					t.Logf("Error closing PTY file: %v", err)
				}
			}
		})
	}
}

func TestNewCommandRunner(t *testing.T) {
	// Test NewCommandRunner function
	runner := NewCommandRunner()

	// Check that returned value is not nil
	if runner == nil {
		t.Error("NewCommandRunner returned nil")
	}

	// Check that returned value is of RealCommandRunner type
	_, ok := runner.(*RealCommandRunner)
	if !ok {
		t.Errorf("NewCommandRunner returned wrong type: %T", runner)
	}
}
