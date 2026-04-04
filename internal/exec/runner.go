package exec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// CommandRunner defines an interface for running commands
type CommandRunner interface {
	RunCommand(command string, args ...string) (string, error)
	StartPTY(command string, args ...string) (*os.File, error)
}

// RealCommandRunner is the CommandRunner implementation that runs actual commands
type RealCommandRunner struct{}

// RunCommand executes the specified command with arguments and returns the output
func (r *RealCommandRunner) RunCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// Return both stdout and stderr in the output string for better error handling
		allOutput := out.String()
		if stderr.Len() > 0 {
			if allOutput != "" {
				allOutput += "\n"
			}
			allOutput += stderr.String()
		}
		return allOutput, fmt.Errorf("command execution error: %v - %s", err, stderr.String())
	}
	return out.String(), nil
}

// StartPTY executes the specified command with arguments using PTY
func (r *RealCommandRunner) StartPTY(command string, args ...string) (*os.File, error) {
	cmd := exec.Command(command, args...)
	return pty.Start(cmd)
}

// NewCommandRunner returns the default CommandRunner implementation
func NewCommandRunner() CommandRunner {
	return &RealCommandRunner{}
}
