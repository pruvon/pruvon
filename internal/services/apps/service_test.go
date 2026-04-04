package apps

import (
	"os"
	"testing"

	"github.com/pruvon/pruvon/internal/dokku"

	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	// Test NewService with nil runner
	service := NewService(nil)
	assert.NotNil(t, service)
	assert.Equal(t, dokku.DefaultCommandRunner, service.CommandRunner())

	// Test NewService with custom runner
	mockRunner := &mockCommandRunner{}
	service = NewService(mockRunner)
	assert.NotNil(t, service)
	assert.Equal(t, mockRunner, service.CommandRunner())
}

// mockCommandRunner implements exec.CommandRunner for testing
type mockCommandRunner struct{}

func (m *mockCommandRunner) RunCommand(name string, args ...string) (string, error) {
	return "mock output", nil
}

func (m *mockCommandRunner) StartPTY(name string, args ...string) (*os.File, error) {
	return nil, nil // Mock implementation
}

func TestRunDokkuCommand(t *testing.T) {
	service := NewService(&mockCommandRunner{})

	result, err := service.RunDokkuCommand("test", "arg1", "arg2")
	assert.NoError(t, err)
	assert.Equal(t, "mock output", result)
}

// Note: More comprehensive tests would require mocking the dokku package
// and testing individual service methods. For now, we test basic functionality.
