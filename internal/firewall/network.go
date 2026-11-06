package firewall

import (
	"errors"
	"io"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/detect"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	Zone   = "home"
	wired  = "802-3-ethernet"
	wifi   = "802-11-wireless"
	fields = 4
)

var laptop = detect.Laptop

type Connection struct {
	UUID   string
	Kind   string
	Device string
	Name   string
}

func Active() []Connection {
	output, _ := run.CaptureQuiet(run.Quick, "nmcli", "-t", "-e", "no", "-f", "UUID,TYPE,DEVICE,NAME", "connection", "show", "--active")
	var found []Connection
	for _, line := range run.Lines(output) {
		parts := strings.SplitN(line, ":", fields)
		if len(parts) == fields && usable(parts[2]) {
			found = append(found, Connection{parts[0], parts[1], parts[2], parts[3]})
		}
	}
	return found
}

func Shown(connections []Connection) (Connection, bool) {
	kinds := []string{wired, wifi}
	if laptop() {
		kinds = []string{wifi, wired}
	}
	for _, kind := range kinds {
		if index := slices.IndexFunc(connections, func(connection Connection) bool { return connection.Kind == kind }); index >= 0 {
			return connections[index], true
		}
	}
	return Connection{}, false
}

func setting(uuid, key string) (string, error) {
	value, err := run.CaptureQuiet(run.Quick, "nmcli", "-g", key, "connection", "show", "uuid", uuid)
	return strings.TrimSpace(value), err
}

func Trusted(uuid string) bool {
	if uuid == "" {
		return false
	}
	zone, err := setting(uuid, "connection.zone")
	return err == nil && strings.EqualFold(zone, Zone)
}

func Trustable(uuid string) error {
	kind, err := setting(uuid, "connection.type")
	switch {
	case err != nil:
		return errors.New("NetworkManager can't say what kind of network it is")
	case kind == wifi:
		return secured(uuid)
	case kind != wired:
		return errors.New("only Wi-Fi and wired networks can be trusted")
	case laptop():
		return errors.New("a laptop's cable can be any network it's plugged into")
	}
	return nil
}

func secured(uuid string) error {
	keys, err := setting(uuid, "802-11-wireless-security.key-mgmt")
	if err != nil || keys == "" || keys == "owe" {
		return errors.New("it's an open network, and anyone can start one with that name")
	}
	return nil
}

func Mark(out io.Writer, connection Connection, trusted bool) (bool, error) {
	zone := ""
	if trusted {
		zone = Zone
	}
	if err := run.LoggedWithin(run.Quick, out, "nmcli", "connection", "modify", "uuid", connection.UUID, "connection.zone", zone); err != nil {
		return false, err
	}
	return run.Quiet(run.Quick, "nmcli", "device", "reapply", connection.Device) == 0, nil
}
