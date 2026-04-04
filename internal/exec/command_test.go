package exec

import (
	"os"
	"testing"
)

func TestGetRandomNumber(t *testing.T) {
	// Test that GetRandomNumber returns a value between 0 and 99999
	for i := 0; i < 10; i++ {
		num := GetRandomNumber()
		if num < 0 || num >= 100000 {
			t.Fatalf("GetRandomNumber returned %d, expected 0 <= num < 100000", num)
		}
	}
}

func TestRunCommandWithExitCode_DelegatesToRunner(t *testing.T) {
	// Test that RunCommandWithExitCode properly delegates to the runner
	mock := &fakeRunner{err: nil}

	output, err := RunCommandWithExitCode(mock, "echo", "hello")
	if err != nil {
		t.Fatalf("RunCommandWithExitCode should not error with fake runner: %v", err)
	}
	if output != "" {
		t.Fatalf("expected empty output from fake runner, got %q", output)
	}
}

// fakeRunner for testing
type fakeRunner struct {
	err error
}

func (f *fakeRunner) RunCommand(command string, args ...string) (string, error) {
	return "", f.err
}

func (f *fakeRunner) StartPTY(command string, args ...string) (*os.File, error) {
	return nil, f.err
}
