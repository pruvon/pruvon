package web

import (
	"github.com/pruvon/pruvon/internal/appdeps"
	"github.com/pruvon/pruvon/internal/dokku"
)

var dokkuRunner dokku.CommandRunner = dokku.DefaultCommandRunner

// InitializeDependencies configures shared dependencies for web handlers.
func InitializeDependencies(deps *appdeps.Dependencies) {
	if deps == nil {
		return
	}
	if deps.DokkuRunner != nil {
		dokkuRunner = deps.DokkuRunner
	}
}
