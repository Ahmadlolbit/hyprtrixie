package firewall

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/Ahmadlolbit/hyprtrixie/internal/apt"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	tag       = "hyprtrixie "
	nameLimit = 15
	ruleWords = 11
)

type Service struct {
	App     string
	Package string
	Ports   []string
}

var Services = []Service{
	{App: "localsend", Package: "localsend", Ports: []string{"53317/tcp", "53317/udp"}},
}

var events = []string{"up", "down", "reapply"}

type Rule struct {
	Device  string
	Port    string
	Proto   string
	Service string
}

func (rule Rule) spec() []string {
	return []string{"allow", "in", "on", rule.Device, "to", "any", "port", rule.Port, "proto", rule.Proto}
}

func (rule Rule) args() []string {
	return append(rule.spec(), "comment", tag+rule.Service)
}

func parsed(line string) (Rule, bool) {
	body, comment, found := strings.Cut(line, " comment '")
	named, closed := strings.CutSuffix(comment, "'")
	service, ours := strings.CutPrefix(named, tag)
	words := strings.Fields(body)
	if !found || !closed || !ours || len(words) != ruleWords || !plain(service) {
		return Rule{}, false
	}
	rule := Rule{Device: words[4], Port: words[8], Proto: words[10], Service: service}
	return rule, body == "ufw "+strings.Join(rule.spec(), " ")
}

func Ours(listing string) []Rule {
	var rules []Rule
	for _, line := range run.Lines(listing) {
		if rule, ok := parsed(line); ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func Opens(app string) bool {
	return slices.ContainsFunc(Services, func(service Service) bool { return service.App == app })
}

func Wanted(device string, present func(string) bool) []Rule {
	var rules []Rule
	for _, service := range Services {
		if !present(service.Package) {
			continue
		}
		for _, port := range service.Ports {
			number, proto, _ := strings.Cut(port, "/")
			rules = append(rules, Rule{device, number, proto, service.App})
		}
	}
	return rules
}

func installedPackages() func(string) bool {
	packages, err := apt.Installed()
	return func(name string) bool {
		return err == nil && packages[name]
	}
}

func odd(letter rune) bool {
	return !unicode.IsLetter(letter) && !unicode.IsDigit(letter) && !strings.ContainsRune("-_.@", letter)
}

func plain(name string) bool {
	return name != "" && strings.IndexFunc(name, odd) < 0
}

func usable(device string) bool {
	return device != "lo" && len(device) <= nameLimit && plain(device)
}

func added() ([]Rule, error) {
	listing, err := run.CaptureQuiet(run.Quick, "ufw", "show", "added")
	if err != nil {
		return nil, fmt.Errorf("ufw can't list its rules: %w", err)
	}
	return Ours(listing), nil
}

func change(out io.Writer, args ...string) error {
	return run.LoggedWithin(run.Quick, out, "ufw", args...)
}

func Sync(out io.Writer, device string, wanted []Rule) error {
	current, err := added()
	if err != nil {
		return err
	}
	var problems []error
	for _, rule := range current {
		if rule.Device == device && !slices.Contains(wanted, rule) {
			problems = append(problems, change(out, append([]string{"delete"}, rule.args()...)...))
		}
	}
	for _, rule := range wanted {
		if !slices.Contains(current, rule) {
			problems = append(problems, change(out, rule.args()...))
		}
	}
	return errors.Join(problems...)
}

func Apply(out io.Writer, device, uuid, event string) error {
	if !usable(device) || !slices.Contains(events, event) {
		return nil
	}
	var wanted []Rule
	if event != "down" && Trusted(uuid) && Trustable(uuid) == nil {
		wanted = Wanted(device, installedPackages())
	}
	return Sync(out, device, wanted)
}

func stale(active []Connection) []string {
	current, _ := added()
	var devices []string
	for _, rule := range current {
		inUse := slices.ContainsFunc(active, func(connection Connection) bool { return connection.Device == rule.Device })
		if !inUse && !slices.Contains(devices, rule.Device) {
			devices = append(devices, rule.Device)
		}
	}
	return devices
}

func networks(connections []Connection) []Connection {
	return slices.DeleteFunc(connections, func(connection Connection) bool { return connection.Kind != wired && connection.Kind != wifi })
}

func Refresh(out io.Writer) error {
	active := networks(Active())
	var problems []error
	for _, connection := range active {
		problems = append(problems, Apply(out, connection.Device, connection.UUID, "up"))
	}
	for _, device := range stale(active) {
		problems = append(problems, Sync(out, device, nil))
	}
	return errors.Join(problems...)
}
