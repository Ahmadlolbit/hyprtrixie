package setup

import (
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/atomic"
	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
	"github.com/Ahmadlolbit/hyprtrixie/internal/runlog"
	"github.com/Ahmadlolbit/hyprtrixie/internal/service"
)

const (
	powerProfiles  = "power-profiles-daemon.service"
	autoUpgrades   = "/etc/apt/apt.conf.d/52hyprtrixie-auto-upgrades"
	networkManager = "NetworkManager"
	loopbackOnly   = "source /etc/network/interfaces.d/*\n\nauto lo\niface lo inet loopback\n"
)

type wifiNetwork struct {
	device string
	name   string
	key    string
}

func interfacesFile() string {
	return env.Or("HYPRTRIXIE_INTERFACES", "/etc/network/interfaces")
}

func powerInstall(log *runlog.Log) error {
	lists := []string{"power"}
	if detect.ReadHardware().CPU == "intel" {
		lists = append(lists, "power-intel")
	}
	if err := apt.InstallLists(log, lists...); err != nil {
		return err
	}
	return run.Logged(log, "systemctl", "enable", powerProfiles)
}

func powerReady(log *runlog.Log) error {
	if err := apt.ListsPresent(log, "power"); err != nil {
		return err
	}
	return expect(service.Enabled(powerProfiles), powerProfiles+" isn't enabled")
}

func firewallSetup(log *runlog.Log) error {
	if err := run.Logged(log, "ufw", "default", "deny", "incoming"); err != nil {
		return err
	}
	if err := run.Logged(log, "ufw", "default", "allow", "outgoing"); err != nil {
		return err
	}
	return run.Logged(log, "ufw", "--force", "enable")
}

func firewallReady(*runlog.Log) error {
	output, _ := run.CaptureQuiet(run.Minute, "ufw", "status")
	return holds(lineHasPrefix(output, "Status: active"))
}

func (context *Context) autoUpdatesSetup(*runlog.Log) error {
	enabled := "0"
	if context.Config.Get("AUTO_UPDATES") == "yes" {
		enabled = "1"
	}
	return context.Deploy.File("system/apt/52hyprtrixie-auto-upgrades", autoUpgrades, "ENABLED="+enabled)
}

func autoUpdatesReady(*runlog.Log) error {
	if err := apt.Present("unattended-upgrades"); err != nil {
		return err
	}
	output, _ := run.CaptureQuiet(run.Minute, "apt-config", "dump", "APT::Periodic::Unattended-Upgrade")
	return expect(strings.Contains(output, `"1";`), "apt has APT::Periodic::Unattended-Upgrade off")
}

func sshServerOff(log *runlog.Log) error {
	if !apt.Installed()["openssh-server"] {
		return nil
	}
	for _, unit := range []string{"ssh.socket", "ssh.service"} {
		run.Logged(io.Discard, "systemctl", "disable", unit)
	}
	log.Line("info", "the SSH server stays off from the next boot")
	return nil
}

func sshServerDisabled(*runlog.Log) error {
	return holds(!service.Enabled("ssh.service") && !service.Enabled("ssh.socket"))
}

func wordsOf(line string) []string {
	words := strings.FieldsFunc(line, func(letter rune) bool { return letter == ' ' || letter == '\t' })
	return append(words, "", "")
}

func beyondLoopback(lines []string) bool {
	return slices.ContainsFunc(lines, func(line string) bool {
		words := wordsOf(line)
		return words[0] == "iface" && words[1] != "lo"
	})
}

func networkName(line string) string {
	rest := strings.TrimPrefix(strings.TrimLeft(line, blanks), "wpa-ssid")
	name := strings.TrimLeft(rest, blanks)
	if name == rest {
		name = line
	}
	return strings.TrimSuffix(strings.TrimPrefix(name, `"`), `"`)
}

func wifiNetworks(lines []string) []wifiNetwork {
	device := ""
	names, keys := map[string]string{}, map[string]string{}
	for _, line := range lines {
		words := wordsOf(line)
		switch words[0] {
		case "iface":
			device = words[1]
		case "wpa-ssid":
			names[device] = networkName(line)
		case "wpa-psk":
			keys[device] = words[1]
		}
	}
	var found []wifiNetwork
	for _, known := range slices.Sorted(maps.Keys(names)) {
		found = append(found, wifiNetwork{known, names[known], keys[known]})
	}
	return found
}

func connectionKnown(name string) bool {
	output, _ := run.CaptureQuiet(run.Minute, "nmcli", "-t", "-e", "no", "-f", "NAME", "connection", "show")
	return slices.Contains(run.Lines(output), name)
}

func addConnection(log *runlog.Log, network wifiNetwork) error {
	if network.name == "" || connectionKnown(network.name) {
		return nil
	}
	args := []string{"connection", "add", "type", "wifi", "ifname", network.device, "con-name", network.name, "ssid", network.name}
	if network.key != "" {
		args = append(args, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", network.key)
	}
	return run.Logged(log, "nmcli", args...)
}

func networkHandover(log *runlog.Log) error {
	path := interfacesFile()
	lines, err := fsutil.Lines(path)
	if err != nil || !beyondLoopback(lines) {
		return err
	}
	if !service.Active(networkManager) {
		if err := run.Logged(log, "systemctl", "start", networkManager); err != nil {
			return err
		}
	}
	for _, network := range wifiNetworks(lines) {
		if err := addConnection(log, network); err != nil {
			return err
		}
	}
	if err := keepOld(log, path); err != nil {
		return err
	}
	if err := atomic.Write(path, []byte(loopbackOnly), plainMode); err != nil {
		return err
	}
	log.Line("info", "NetworkManager takes over the network connections from the next boot")
	return nil
}

func networkReady(*runlog.Log) error {
	lines, err := fsutil.Lines(interfacesFile())
	if err != nil {
		return err
	}
	return expect(!beyondLoopback(lines), interfacesFile()+" still has connections")
}

func securitySteps(context *Context) {
	settings := context.Config
	context.Run.Step("Set up power profiles", powerInstall)
	if settings.Get("FIREWALL") == "yes" {
		context.Run.Step("Turn on the firewall", firewallSetup)
	}
	context.Run.Step("Set up automatic security updates", context.autoUpdatesSetup)
	if settings.Get("SSH_POLICY") == "client" {
		context.Run.Step("Keep the SSH server off", sshServerOff)
	}
	if settings.Get("NM_HANDOVER") == "yes" {
		context.Run.Step("Hand the network over to NetworkManager", networkHandover)
	}
}

func securityChecks(context *Context) {
	settings := context.Config
	if settings.Get("FIREWALL") == "yes" {
		context.Run.Check("The firewall is on", firewallReady)
	}
	if settings.Get("NM_HANDOVER") == "yes" {
		context.Run.Check("NetworkManager manages every connection", networkReady)
	}
	context.Run.Check("Power profiles are available", powerReady)
	if settings.Get("SSH_POLICY") == "client" {
		context.Run.Check("The SSH server is off", sshServerDisabled)
	}
	if settings.Get("AUTO_UPDATES") == "yes" {
		context.Run.Check("Security updates install automatically", autoUpdatesReady)
	}
}
