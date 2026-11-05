package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/account"
	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/files"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ledger"
	"github.com/Ahmadlolbit/hyprtrixie/internal/modules"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
	"github.com/Ahmadlolbit/hyprtrixie/internal/ui"
)

const shownNames = 8

var undoable = []string{"09", "10", "11", "16"}

type undoPlan struct {
	id       string
	packages []string
	services []string
	files    []string
	made     []string
}

func moduleTitle(id string) string {
	for _, module := range modules.All {
		if module.ID == id {
			return module.Title
		}
	}
	return id
}

func planUndo(id string) undoPlan {
	record := ledger.Load(id)
	elsewhere := ledger.WantedElsewhere(id)
	shared := ledger.Elsewhere(id, ledger.File)
	installed := apt.Installed()
	plan := undoPlan{id: id, services: record.Values(ledger.Service)}
	for _, path := range record.Values(ledger.File) {
		if !shared[path] {
			plan.files = append(plan.files, path)
		}
	}
	for _, name := range record.Values(ledger.Package) {
		if installed[name] && !elsewhere[name] {
			plan.packages = append(plan.packages, name)
		}
	}
	for _, path := range record.Values(ledger.Made) {
		if removable(path) && fsutil.Exists(path) {
			plan.made = append(plan.made, path)
		}
	}
	return plan
}

func removable(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(clean) && strings.Count(clean, "/") >= 2 && clean != filepath.Clean(env.Home())
}

func named(noun string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	shown := names[:min(len(names), shownNames)]
	text := fmt.Sprintf("  %d %s: %s", len(names), noun, strings.Join(shown, " "))
	if len(names) > len(shown) {
		text += fmt.Sprintf(" and %d more", len(names)-len(shown))
	}
	return text + "\n"
}

func (plan undoPlan) describe() string {
	text := fmt.Sprintf("Undoing module %s, %s, takes back:\n", plan.id, moduleTitle(plan.id))
	text += named("packages", plan.packages) + named("services", plan.services) + named("files", plan.files) + named("unpacked", plan.made)
	return text + "Packages and files another module also uses stay, and so do files you've changed since. apt also removes\n" +
		"other packages nothing needs any more, as apt autoremove does.\n"
}

func snapshotBefore(description string) error {
	if !installed("snapper") || !fsutil.IsFile(env.Or("HYPRTRIXIE_SNAPPER_CONFIG", "/etc/snapper/configs/root")) {
		fmt.Println("Snapshots aren't set up, so there's no restore point.")
		return nil
	}
	fmt.Println("Taking a restore point first")
	if run.Status("snapper", "--no-dbus", "-c", "root", "create", "--description", description, "--cleanup-algorithm", "number") != 0 {
		return errors.New("the restore point failed, so nothing was changed")
	}
	return nil
}

func (plan undoPlan) takeFiles(log *runlog.Log) error {
	deployer, err := files.New(log, account.Account{})
	if err != nil {
		return err
	}
	for _, path := range plan.files {
		outcome, err := deployer.Undeploy(path)
		if err != nil {
			return err
		}
		if outcome == "kept" {
			fmt.Fprintln(log, "kept "+path+": you changed it since")
		}
	}
	return nil
}

func (plan undoPlan) takeMade(log *runlog.Log) error {
	for _, path := range plan.made {
		fmt.Fprintln(log, "removing "+path)
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func (plan undoPlan) run(log *runlog.Log) error {
	steps := []func(*runlog.Log) error{
		func(log *runlog.Log) error { return service.Disable(log, plan.services...) },
		plan.takeMade,
		plan.takeFiles,
		func(log *runlog.Log) error { return apt.Unneeded(log, plan.packages...) },
	}
	for _, step := range steps {
		if err := step(log); err != nil {
			return err
		}
	}
	if err := ledger.Drop(plan.id); err != nil {
		return err
	}
	return modules.SaveStatus(map[string]string{plan.id: modules.Undone})
}

func undo(id string, yes bool) int {
	if !ledger.Known(id) {
		return fail("There's no record of module %s yet. Run the setup once (sudo hyprtrixie install), then undo it.", id)
	}
	plan := planUndo(id)
	fmt.Print(plan.describe())
	if !yes && !ui.Confirm("Undo module "+id+"?") {
		return fail("Nothing was changed.")
	}
	if err := snapshotBefore("before undoing module " + id); err != nil {
		return fail("%v", err)
	}
	log := runlog.Joined()
	log.Mirror(func(data []byte) { os.Stdout.Write(data) })
	if err := plan.run(log); err != nil {
		return fail("Undoing module %s stopped: %v. The restore point taken first can bring the system back: hyprtrixie restore.", id, err)
	}
	fmt.Printf("Module %s is undone. The setup leaves it out until you run it again: sudo hyprtrixie install --only %s\n", id, id)
	return 0
}

func undoCommand() int {
	id, yes := env.Arg(2), env.Arg(3) == "--yes"
	if id == "" || (env.Arg(3) != "" && !yes) || env.Arg(4) != "" {
		return fail("usage: hyprtrixie undo MODULE [--yes], where MODULE is 09, 10, 11 or 16")
	}
	if !slices.Contains(undoable, id) {
		return fail("Module %s can't be undone: only 09, 10, 11 and 16 can, because the others hold up the desktop or the boot.", id)
	}
	return asRoot("undo "+id, func() int { return undo(id, yes) })
}
