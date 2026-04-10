package dokku

import (
	"os"

	execpkg "github.com/pruvon/pruvon/internal/exec"
)

// OsReadFile, os.ReadFile fonksiyonunu test için değiştirmek üzere kullanılacak
var OsReadFile = os.ReadFile

// OsStat, os.Stat fonksiyonunu test için değiştirmek üzere kullanılacak
var OsStat = os.Stat

// CommandRunner interface for command execution - imported from internal/exec
type CommandRunner = execpkg.CommandRunner

// DefaultCommandRunner is the default command runner instance
var DefaultCommandRunner CommandRunner = execpkg.NewCommandRunner()

func dokkuShellPrefix() string {
	return "sudo -n -u dokku dokku"
}
