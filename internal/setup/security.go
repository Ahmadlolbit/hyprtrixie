package setup

import (
	"errors"
	"io"
	"maps"
	"os"
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
	batteryUnit    = "hyprtrixie-battery.service"
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

func (context *Context) powerInstall(log *runlog.Log) error {
	if err := context.Deploy.File("system/systemd/"+batteryUnit, unitFolder+batteryUnit); err != nil {
		return err
	}
	if err := run.Logged(log, "systemctl", "enable", batteryUnit); err != nil {
		return err
	}
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
	if installed, _ := apt.Installed(); !installed["openssh-server"] {
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
			keys[device] = networkKey(line)
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

func addConnection(network wifiNetwork) (bool, error) {
	if network.name == "" || connectionKnown(network.name) {
		return false, nil
	}
	if err := os.MkdirAll(connectionsFolder(), folderOfSecrets); err != nil {
		return false, err
	}
	return true, atomic.Install(connectionFile(network.device), []byte(keyfile(network)), connectionsMode)
}

func addConnections(log *runlog.Log, networks []wifiNetwork) error {
	added := false
	for _, network := range networks {
		wrote, err := addConnection(network)
		if err != nil {
			return err
		}
		added = added || wrote
	}
	if !added {
		return nil
	}
	return run.LoggedWithin(run.Minute, log, "nmcli", "connection", "reload")
}

func networkHandover(log *runlog.Log) error {
	path := interfacesFile()
	lines, err := fsutil.Lines(path)
	if err != nil || !beyondLoopback(lines) {
		return err
	}
	if reasons := unconvertible(lines); len(reasons) > 0 {
		log.Line("warn", "NetworkManager doesn't take over, because "+path+" has what it can't convert: "+strings.Join(reasons, "; "))
		return nil
	}
	if !service.Active(networkManager) {
		if err := run.Logged(log, "systemctl", "start", networkManager); err != nil {
			return err
		}
	}
	if err := addConnections(log, wifiNetworks(lines)); err != nil {
		return err
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
	if err != nil || !beyondLoopback(lines) {
		return err
	}
	if reasons := unconvertible(lines); len(reasons) > 0 {
		return errors.New(interfacesFile() + " keeps what NetworkManager can't take over: " + strings.Join(reasons, "; "))
	}
	return errors.New(interfacesFile() + " still has connections")
}

func securitySteps(context *Context) {
	settings := context.Config
	context.Run.Step("Set up power profiles", context.powerInstall)
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
	if settings.Get("FINGERPRINT") == "yes" {
		context.Run.Step("Use your finger for sudo and the lock screen", fingerprintSetup)
	} else if fingerprintInSudo() {
		context.Run.Step("Take your finger out of sudo", fingerprintOff)
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
	if settings.Get("FINGERPRINT") == "yes" {
		context.Run.Check("sudo takes your finger", fingerprintReady)
	}
}
