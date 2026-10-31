package setup

import (
	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

type Context struct {
	Run    *step.Runner
	Deploy *files.Deployer
	Person account.Account
	Config *config.Config
}

type Module struct {
	Steps  func(*Context)
	Checks func(*Context)
}

var Modules = map[string]Module{
	"09": {cliSteps, cliChecks},
	"16": {backupSteps, backupChecks},
}
