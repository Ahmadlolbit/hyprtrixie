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
	CanAsk bool
}

type Module struct {
	Steps  func(*Context)
	Checks func(*Context)
}

var Modules map[string]Module

func init() {
	Modules = map[string]Module{
		"00": {initSteps, initChecks},
		"01": {preflightSteps, preflightChecks},
		"02": {repositorySteps, repositoryChecks},
		"03": {kernelSteps, kernelChecks},
		"04": {swapSteps, swapChecks},
		"05": {snapshotSteps, snapshotChecks},
		"06": {hyprlandSteps, hyprlandChecks},
		"07": {sddmSteps, sddmChecks},
		"08": {terminalSteps, terminalChecks},
		"09": {cliSteps, cliChecks},
		"10": {appSteps, appChecks},
		"11": {developerSteps, developerChecks},
		"12": {sessionSteps, sessionChecks},
		"13": {barSteps, barChecks},
		"14": {themeSteps, themeChecks},
		"15": {securitySteps, securityChecks},
		"16": {backupSteps, backupChecks},
		"17": {reportSteps, noChecks},
	}
}
