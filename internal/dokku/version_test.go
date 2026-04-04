package dokku

import (
	"errors"
	"testing"
)

func TestGetDokkuVersion(t *testing.T) {
	tests := []struct {
		name           string
		mockOutput     string
		mockError      error
		expectedResult string
		expectedError  bool
	}{
		{
			name:           "Valid version output",
			mockOutput:     "dokku version 0.34.4",
			mockError:      nil,
			expectedResult: "0.34.4",
			expectedError:  false,
		},
		{
			name:           "Valid version output with extra spaces",
			mockOutput:     "dokku  version  0.35.0",
			mockError:      nil,
			expectedResult: "0.35.0",
			expectedError:  false,
		},
		{
			name:           "Invalid version output format",
			mockOutput:     "dokku 0.34.4",
			mockError:      nil,
			expectedResult: "dokku 0.34.4",
			expectedError:  false,
		},
		{
			name:           "Single word output fallback",
			mockOutput:     "0.34.4",
			mockError:      nil,
			expectedResult: "0.34.4",
			expectedError:  false,
		},
		{
			name:           "Command execution error",
			mockOutput:     "",
			mockError:      errors.New("command failed"),
			expectedResult: "",
			expectedError:  true,
		},
		{
			name:           "Empty output",
			mockOutput:     "",
			mockError:      nil,
			expectedResult: "",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputMap := make(map[string]string)
			errorMap := make(map[string]error)

			if tt.mockError != nil {
				errorMap["dokku --version"] = tt.mockError
			} else {
				outputMap["dokku --version"] = tt.mockOutput
			}

			mockRunner := &MockCommandRunner{
				OutputMap: outputMap,
				ErrorMap:  errorMap,
			}

			result, err := GetDokkuVersion(mockRunner)

			if tt.expectedError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if result != tt.expectedResult {
				t.Errorf("Expected result %q but got %q", tt.expectedResult, result)
			}
		})
	}
}

func TestGetDokkuVersion_UsesDefaultRunnerValue(t *testing.T) {
	// Save the original DefaultCommandRunner
	originalRunner := DefaultCommandRunner
	defer func() {
		DefaultCommandRunner = originalRunner
	}()

	// Set up a mock runner
	mockRunner := &MockCommandRunner{
		OutputMap: map[string]string{
			"dokku --version": "dokku version 0.34.4",
		},
		ErrorMap: map[string]error{},
	}
	DefaultCommandRunner = mockRunner

	result, err := GetDokkuVersion(DefaultCommandRunner)

	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if result != "0.34.4" {
		t.Errorf("Expected result %q but got %q", "0.34.4", result)
	}
}
