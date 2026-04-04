package dokku

import (
	"errors"
	"os"
	"strings"
)

// MockCommandRunner implements the CommandRunner interface for testing
type MockCommandRunner struct {
	OutputMap map[string]string
	ErrorMap  map[string]error
}

func (m *MockCommandRunner) RunCommand(command string, args ...string) (string, error) {
	key := command + " " + strings.Join(args, " ")

	if err, exists := m.ErrorMap[key]; exists {
		return "", err
	}

	if output, exists := m.OutputMap[key]; exists {
		return output, nil
	}

	return "", errors.New("command not mocked: " + key)
}

func (m *MockCommandRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, errors.New("PTY not supported in mock")
}
