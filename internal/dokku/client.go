package dokku

import (
	"os"

	execpkg "github.com/pruvon/pruvon/internal/exec"
)

// OsReadFile is used to replace os.ReadFile for testing
var OsReadFile = os.ReadFile

// OsStat is used to replace os.Stat for testing
var OsStat = os.Stat

// CommandRunner interface for command execution - imported from internal/exec
type CommandRunner = execpkg.CommandRunner

// DefaultCommandRunner is the default command runner instance
var DefaultCommandRunner CommandRunner = execpkg.NewCommandRunner()

func dokkuShellPrefix() string {
	return "sudo -n -u dokku dokku"
}
