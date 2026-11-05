package osinstall

import (
	"slices"

	"github.com/Ahmadlolbit/hyprtrixie/internal/disks"
	"github.com/Ahmadlolbit/hyprtrixie/internal/step"
)

func Steps(runner *step.Runner, answers Answers, plan disks.Plan) {
	work := &install{answers, plan}
	runner.Step("Start from a clean slate", work.clean)
	for _, group := range Grouped(slices.Concat(plan.Partition(), disks.Layout(), plan.Boot())) {
		runner.Step(group[0].Title, work.commands(group))
	}
	runner.Step("Copy the system", work.copySystem)
	runner.Step("Write fstab and crypttab", work.tables)
	runner.Step("Open the new system", work.bind)
	runner.Step("Take out the live system's parts", work.purgeLive)
	runner.Step("Make your account", work.account)
	runner.Step("Name the computer", work.name)
	runner.Step("Set the time zone, keyboard and language", work.localize)
	runner.Step("Give the computer its own identity", work.identity)
	runner.Step("Build the boot image", work.bootImage)
	runner.Step("Install the boot loader", work.bootLoader)
	runner.Step("Hand the rest of the setup to the first start", work.firstBoot)
	runner.Step("Close the new system", work.close)
}
