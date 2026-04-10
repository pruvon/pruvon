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

var sudoEligibleCommands = map[string]struct{}{
	"chmod": {},
	"chown": {},
}

func shouldUseSudo(command string) bool {
	if os.Geteuid() == 0 {
		return false
	}

	if os.Getenv("PRUVON_DISABLE_SUDO") == "1" {
		return false
	}

	_, ok := sudoEligibleCommands[command]
	return ok
}

func buildDokkuCommand(args ...string) *exec.Cmd {
	sudoArgs := append([]string{"-n", "-u", "dokku", "dokku"}, args...)
	return exec.Command("sudo", sudoArgs...)
}

func buildCommand(command string, args ...string) *exec.Cmd {
	if command == "dokku" {
		return buildDokkuCommand(args...)
	}

	if shouldUseSudo(command) {
		sudoArgs := append([]string{"-n", command}, args...)
		return exec.Command("sudo", sudoArgs...)
	}

	return exec.Command(command, args...)
}

// RunCommand executes the specified command with arguments and returns the output
func (r *RealCommandRunner) RunCommand(command string, args ...string) (string, error) {
	cmd := buildCommand(command, args...)
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
	cmd := buildCommand(command, args...)
	return pty.Start(cmd)
}

// NewCommandRunner returns the default CommandRunner implementation
func NewCommandRunner() CommandRunner {
	return &RealCommandRunner{}
}
