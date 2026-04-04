package exec

import (
	"math/rand"
	"time"
)

// Initialize random source for generation
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// GetRandomNumber returns a random integer
func GetRandomNumber() int {
	return rng.Intn(100000)
}

// RunCommandWithExitCode runs a command and returns both output and whether it was successful
func RunCommandWithExitCode(runner CommandRunner, command string, args ...string) (string, error) {
	// Run the command normally
	output, err := runner.RunCommand(command, args...)

	// Return the output and error (which indicates exit code status)
	return output, err
}
