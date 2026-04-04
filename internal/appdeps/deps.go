package appdeps

import (
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/dokku"
	"github.com/pruvon/pruvon/internal/exec"
	appsvc "github.com/pruvon/pruvon/internal/services/apps"
)

// Dependencies contains the shared runtime dependencies for handlers.
type Dependencies struct {
	Config      *config.Config
	DokkuRunner dokku.CommandRunner
	ExecRunner  exec.CommandRunner
	AppService  *appsvc.Service
}

// NewDependencies constructs the shared handler dependencies.
func NewDependencies(cfg *config.Config) *Dependencies {
	dokkuRunner := dokku.DefaultCommandRunner
	execRunner := exec.NewCommandRunner()

	return &Dependencies{
		Config:      cfg,
		DokkuRunner: dokkuRunner,
		ExecRunner:  execRunner,
		AppService:  appsvc.NewService(dokkuRunner),
	}
}
