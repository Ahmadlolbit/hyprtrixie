package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apps"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/config"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/disks"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/hypr"
	"github.com/Ahmadlolbit/hyprtrixie/internal/keyboard"
	"github.com/Ahmadlolbit/hyprtrixie/internal/osinstall"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	shortest = 8
	diskAt   = 1
	youAt    = 3
)

type system struct {
	answers osinstall.Answers
	twice   string
	again   string
	found   []disks.Disk
	room    map[string]string
	reuse   map[string]bool
	blocked string
	seen    string
	say     func(string)
}

func answersFile() string {
	return env.Or("HYPRTRIXIE_ANSWERS", "/run/hyprtrixie-install.json")
}

func zoneFolder() string {
	return env.Or("HYPRTRIXIE_ZONEINFO", "/usr/share/zoneinfo")
}

func newSystem(found []disks.Disk) *system {
	zone := strings.TrimSpace(fsutil.Optional("/etc/timezone"))
	if zone == "" {
		zone = "UTC"
	}
	layout := keyboard.Current()
	return &system{found: found, room: map[string]string{}, reuse: map[string]bool{}, seen: layout,
		answers: osinstall.Answers{Host: "hyprtrixie", Zone: zone, Keyboard: layout, Locale: "en_US.UTF-8"}}
}

func (state *system) chosen() (disks.Disk, bool) {
	for _, disk := range state.found {
		if disk.Path == state.answers.Disk {
			return disk, true
		}
	}
	return disks.Disk{}, false
}

func (state *system) roomBeside(disk disks.Disk) string {
	if why, known := state.room[disk.Path]; known {
		return why
	}
	probe := state.answers
	probe.Disk, probe.Beside = disk.Path, true
	why := ""
	plan, err := osinstall.PlanFor(probe)
	if err != nil {
		why = err.Error()
	}
	state.room[disk.Path], state.reuse[disk.Path] = why, len(plan.Reuses) > 0
	return why
}

func (state *system) pick(disk disks.Disk) {
	state.answers.Disk = disk.Path
	state.answers.DiskID = disk.ID()
	state.answers.Beside = disk.Windows() && state.roomBeside(disk) == ""
}

func (state *system) diskRow(flow *wizard, disk disks.Disk) row {
	mark, chosen := "( )", state.answers.Disk == disk.Path
	if chosen {
		mark = "(*)"
	}
	return row{mark: mark, label: disk.Name(), chosen: chosen, help: "Space picks it, Enter picks it and goes on",
		space: func() tea.Cmd { state.pick(disk); return nil },
		enter: func() tea.Cmd { state.pick(disk); return flow.forward() }}
}

func (state *system) besideRow(disk disks.Disk) row {
	value := "no"
	if state.answers.Beside {
		value = "yes"
	}
	return row{label: "Keep Windows", value: value, help: "Yes puts Hyprtrixie in the free space next to Windows; no erases the whole disk", space: func() tea.Cmd {
		if state.answers.Beside || state.roomBeside(disk) == "" {
			state.answers.Beside = !state.answers.Beside
		}
		return nil
	}}
}

func (state *system) consequences() []row {
	disk, found := state.chosen()
	if !found {
		return []row{{label: "Pick a disk with Space or Enter.", info: true}}
	}
	if state.answers.Beside {
		rows := []row{state.besideRow(disk), {info: true},
			{label: "Windows stays. Hyprtrixie goes in the free space, and the boot menu offers both for five seconds.", info: true}}
		if state.reuse[disk.Path] {
			rows = append(rows, row{label: "An install that stopped halfway left two partitions here: they go, and Hyprtrixie uses their space.", info: true})
		}
		return append(rows, row{label: "Before you go on: save your BitLocker recovery key somewhere off this computer, and turn off Fast Startup in Windows.", info: true})
	}
	rows := []row{{label: "Everything on " + disk.Name() + " is erased.", info: true}}
	if !disk.Windows() {
		return rows
	}
	rows = append(rows, state.besideRow(disk))
	if why := state.roomBeside(disk); why != "" {
		rows = append(rows, row{label: "To keep Windows: " + why + ". In Windows, Disk Management's Shrink Volume on C: makes room; then start this again.", info: true})
	}
	return rows
}

func (state *system) diskRows(flow *wizard) []row {
	rows := []row{{label: "Where Hyprtrixie goes", heading: true}, {info: true}}
	if len(state.found) == 0 {
		return append(rows, row{label: "No disk here can take it: each is under 40 GB, read-only, or in use.", info: true})
	}
	for _, disk := range state.found {
		rows = append(rows, state.diskRow(flow, disk))
	}
	return append(append(rows, row{info: true}), state.consequences()...)
}

func (state *system) diskReady() string {
	if _, found := state.chosen(); !found {
		return "Pick a disk first"
	}
	return ""
}

func masked(text string) string {
	if text == "" {
		return ""
	}
	return "********"
}

func secretRow(label, help string, into *string) row {
	return row{label: label, value: masked(*into), help: help,
		input: &field{allow: anything, valid: func(string) bool { return true }, set: func(text string) { *into = text }, secret: true}}
}

func printable(character rune) bool {
	return character >= ' ' && character <= '~'
}

func bootTyped(line row) row {
	line.input.allow = printable
	return line.refusing("Only English letters, digits and signs: the boot prompt may misread others")
}

func textRow(label, help string, allow func(rune) bool, into *string, after func()) row {
	return row{label: label, value: *into, help: help, input: &field{allow: allow, valid: func(text string) bool { return text != "" }, set: func(text string) {
		*into = text
		if after != nil {
			after()
		}
	}}}
}

func (flow *wizard) lockRows() []row {
	state := flow.system
	state.followLayout()
	return []row{{label: "The disk password", heading: true}, {info: true},
		{label: "Every start asks for it first. Without it nothing on the disk can be read, so keep it somewhere safe.", info: true}, {info: true},
		bootTyped(secretRow("Disk password", "At least 8 characters: English letters, digits, signs and spaces", &state.answers.Password)),
		bootTyped(secretRow("Again", "The same once more", &state.twice)),
		{label: "Words above it", value: flow.settings.Get("BOOT_PROMPT"), help: "What the boot splash shows above the disk password, such as Welcome back; empty keeps the system's own words",
			input: &field{allow: config.SafeText, valid: func(string) bool { return true }, set: func(text string) { flow.settings.Set("BOOT_PROMPT", text) }, refused: safeRefused}},
	}
}

func (state *system) lockReady() string {
	switch {
	case utf8.RuneCountInString(state.answers.Password) < shortest:
		return "The disk password needs at least 8 characters"
	case state.answers.Password != state.twice:
		return "The two disk passwords aren't the same"
	}
	return ""
}

func only(allowed string) func(rune) bool {
	return func(character rune) bool { return strings.ContainsRune(allowed, character) }
}

var (
	lower           = "abcdefghijklmnopqrstuvwxyz0123456789"
	userCharacter   = only(lower + "_-")
	hostCharacter   = only(lower + "-")
	zoneCharacter   = only(lower + "ABCDEFGHIJKLMNOPQRSTUVWXYZ/_+-")
	layoutCharacter = only(lower + "_-")
)

func nameCharacter(character rune) bool {
	return anything(character) && character != ':' && character != ','
}

func (state *system) suggestUser() {
	if state.answers.User != "" {
		return
	}
	first, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(state.answers.Name)), " ")
	state.answers.User = strings.Map(func(character rune) rune {
		if userCharacter(character) {
			return character
		}
		return -1
	}, first)
}

func (flow *wizard) youRows() []row {
	state := flow.system
	state.followLayout()
	return []row{{label: "You and this computer", heading: true}, {info: true},
		textRow("Your name", "Shown on the login screen", nameCharacter, &state.answers.Name, state.suggestUser).refusing("Your name can't have : or , in it"),
		textRow("User name", "For logging in: small letters, digits, - and _", userCharacter, &state.answers.User, nil).lowered().refusing("A user name takes only small letters, digits, - and _"),
		secretRow("Password", "For logging in and for sudo", &state.answers.Secret),
		secretRow("Again", "The same once more", &state.again),
		textRow("Computer's name", "How it shows on the network: small letters, digits and -", hostCharacter, &state.answers.Host, nil).lowered().refusing("The computer's name takes only small letters, digits and -"),
		textRow("Time zone", "Such as Europe/Istanbul or America/New_York", zoneCharacter, &state.answers.Zone, nil).refusing("A time zone takes only letters, digits, slashes and _ + -"),
	}
}

func sentence(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(first)) + text[size:]
}

func (state *system) youReady() string {
	if state.answers.Secret != state.again {
		return "The two passwords aren't the same"
	}
	if err := state.answers.Check(); err != nil {
		return sentence(err.Error())
	}
	if !fsutil.IsFile(filepath.Join(zoneFolder(), state.answers.Zone)) {
		return "There's no time zone " + state.answers.Zone
	}
	return ""
}

func (flow *wizard) systemRows() []row {
	if flow.system == nil {
		return nil
	}
	state := flow.system
	disk, _ := state.chosen()
	where := "erases " + disk.Name()
	if state.answers.Beside {
		where = "next to Windows on " + disk.Name()
	}
	return []row{
		{label: "Disk", value: where, help: "Enter opens this window", enter: func() tea.Cmd { flow.reviewing = true; flow.jump(diskAt); return nil }},
		{label: "You", value: state.answers.User + " on " + state.answers.Host, help: "Enter opens this window", enter: func() tea.Cmd { flow.reviewing = true; flow.jump(youAt); return nil }},
	}
}

func (state *system) installRow(flow *wizard) row {
	return row{label: "Install", help: "Install Hyprtrixie with these choices", enter: func() tea.Cmd {
		for index, ready := range []func() string{state.welcomeReady, state.diskReady, state.lockReady, state.youReady} {
			if why := ready(); why != "" {
				flow.notice = why
				flow.jump(index)
				return nil
			}
		}
		disk, _ := state.chosen()
		question := "Erase " + disk.Name() + " and install Hyprtrixie?"
		if state.answers.Beside {
			question = "Install Hyprtrixie next to Windows on " + disk.Name() + "?"
		}
		box := confirmation(question, flow.install.begin)
		box.cursor = 1
		return flow.open(box)
	}}
}

func (state *system) save(config string) error {
	answers := state.answers
	answers.Config = config
	data, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	return atomic.Install(answersFile(), data, 0o600)
}

func (state *system) arguments(string) []string {
	return []string{"install-os", "--events", "--answers", answersFile()}
}

func desktopReload() ([]string, bool) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		return []string{"hyprctl", "reload"}, true
	}
	if os.Getenv("TERM") == "linux" {
		return nil, true
	}
	uid := os.Getenv("SUDO_UID")
	if uid == "" {
		return nil, false
	}
	hyprctl, found := hypr.Session(filepath.Join(env.Or("HYPRTRIXIE_RUN_USER", "/run/user"), uid))
	if !found {
		return nil, false
	}
	return append(hyprctl, "reload"), true
}

func applyLayout(layout string) string {
	reload, reachable := desktopReload()
	if !reachable {
		return "The keyboard stays as it is: this terminal can't reach the desktop. Open Install Hyprtrixie from the launcher to change it"
	}
	text := keyboard.With(fsutil.Optional(keyboard.File()), layout)
	if err := atomic.Write(keyboard.File(), []byte(text), 0o644); err != nil {
		return "The keyboard stays as it is: " + err.Error()
	}
	if reload == nil {
		run.Quiet(run.Slow, "setupcon", "-k", "--force")
		return ""
	}
	run.Quiet(run.Quick, reload[0], reload[1:]...)
	return ""
}

func (state *system) keyboardRow() row {
	line := textRow("Keyboard", "Such as us, de or gb. It changes at once, so the passwords you type next work at every start", layoutCharacter, &state.answers.Keyboard, nil)
	line.input.refused = "A layout takes only small letters, digits, - and _, such as us or de"
	line.input.set = func(text string) {
		if !keyboard.Known(text) {
			state.tell("There's no layout " + text)
			return
		}
		if why := applyLayout(text); why != "" {
			state.tell(why)
			return
		}
		state.takeLayout(text)
		state.seen = keyboard.Current()
	}
	return line
}

func (state *system) tell(text string) {
	if state.say != nil {
		state.say(text)
	}
}

func (state *system) takeLayout(layout string) {
	if layout != state.answers.Keyboard {
		state.answers.Password, state.twice, state.answers.Secret, state.again = "", "", "", ""
	}
	state.answers.Keyboard = layout
}

func (state *system) followLayout() {
	if current := keyboard.Current(); current != state.seen {
		state.seen = current
		if keyboard.Known(current) {
			state.takeLayout(current)
		}
	}
}

func (state *system) welcomeReady() string {
	return state.blocked
}

func (state *system) welcomeRows(machine []string) []row {
	state.followLayout()
	rows := []row{{label: "Install Hyprtrixie", heading: true}, {info: true},
		{label: "Debian 13 with Hyprland, set up properly.", info: true},
		{label: "It takes about ten minutes and needs no internet.", info: true},
		{label: "Nothing changes until you choose Install at the end.", info: true},
		{info: true}}
	if state.blocked != "" {
		rows = append(rows, row{label: state.blocked, info: true}, row{info: true})
	}
	rows = append(rows, state.keyboardRow(), row{info: true})
	for _, line := range machine {
		rows = append(rows, row{label: line, info: true})
	}
	return rows
}

func (flow *wizard) systemStages(machine []string, catalogue []apps.App) []stage {
	state := flow.system
	state.say = func(text string) { flow.notice = text }
	stages := []stage{
		&welcome{page: &page{wizard: flow, title: "Welcome", rows: func() []row { return state.welcomeRows(machine) }, ready: state.welcomeReady}, started: time.Now()},
		&page{wizard: flow, title: "Disk", rows: func() []row { return state.diskRows(flow) }, ready: state.diskReady},
		&page{wizard: flow, title: "Disk password", rows: flow.lockRows, ready: state.lockReady},
		&page{wizard: flow, title: "You", rows: flow.youRows, ready: state.youReady},
	}
	flow.offset = len(stages) - 1
	flow.settings.Set(kernelKey, liveKernel)
	return append(stages, flow.allStages(machine, catalogue)[1:]...)
}

func RunInstaller() int {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "the installer runs as root: open Install Hyprtrixie from the app launcher, or start it with sudo in a terminal on the desktop")
		return 1
	}
	if !canShow() {
		fmt.Fprintln(os.Stderr, "the installer needs a terminal")
		return 1
	}
	found, err := osinstall.Disks()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flow := newWizard(config.Read(config.Path()))
	flow.show = canShow
	flow.nvidia = detect.ReadHardware().HasGPU("nvidia") || detect.NvidiaOnBus()
	flow.system = newSystem(found)
	if err := osinstall.Ready(); err != nil {
		flow.system.blocked = sentence(err.Error())
	}
	flow.stages = flow.systemStages(run.Lines(detect.Summary()), apps.All())
	return play(flow)
}
