package dokku

import (
	"errors"
	"testing"
)

func TestGetRedirects(t *testing.T) {
	tests := []struct {
		name              string
		output            string
		commandError      error
		expectedRedirects int
		expectedError     bool
	}{
		{
			name: "multiple redirects",
			output: `SOURCE                DESTINATION            CODE
/old-path            /new-path              301
/another-path        /different-path        302`,
			expectedRedirects: 2,
			expectedError:     false,
		},
		{
			name: "single redirect",
			output: `SOURCE                DESTINATION            CODE
/old                 /new                   301`,
			expectedRedirects: 1,
			expectedError:     false,
		},
		{
			name:              "no redirects",
			output:            "SOURCE                DESTINATION            CODE",
			expectedRedirects: 0,
			expectedError:     false,
		},
		{
			name:              "command error",
			output:            "",
			commandError:      errors.New("redirect plugin not installed"),
			expectedRedirects: 0,
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &MockCommandRunner{
				OutputMap: map[string]string{},
				ErrorMap:  map[string]error{},
			}

			if tt.commandError != nil {
				mockRunner.ErrorMap["dokku redirect testapp"] = tt.commandError
			} else {
				mockRunner.OutputMap["dokku redirect testapp"] = tt.output
			}

			redirects, err := GetRedirects(mockRunner, "testapp")

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(redirects) != tt.expectedRedirects {
				t.Errorf("Expected %d redirects, got %d", tt.expectedRedirects, len(redirects))
			}
		})
	}
}
