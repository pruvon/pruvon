package api

import (
	"github.com/pruvon/pruvon/internal/appdeps"
	"github.com/pruvon/pruvon/internal/dokku"
	appsvc "github.com/pruvon/pruvon/internal/services/apps"
)

var commandRunner dokku.CommandRunner = dokku.DefaultCommandRunner

var appService = appsvc.NewService(commandRunner)

func initializeDependencies(deps *appdeps.Dependencies) {
	if deps == nil {
		return
	}
	if deps.DokkuRunner != nil {
		commandRunner = deps.DokkuRunner
	}
	if deps.AppService != nil {
		appService = deps.AppService
	}
}
