package setup

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

const (
	connectionsMode = 0o600
	folderOfSecrets = 0o700
)

var (
	stanzaWords   = []string{"auto", "source", "source-directory", "mapping", "rename", "no-auto-down", "no-scripts"}
	handedOptions = []string{"wpa-ssid", "wpa-psk"}
	keyfileEscape = strings.NewReplacer(`\`, `\\`, "\t", `\t`, "\r", `\r`, "\n", `\n`)
)

func connectionsFolder() string {
	return env.Or("HYPRTRIXIE_NM_CONNECTIONS", "/etc/NetworkManager/system-connections")
}

func connectionFile(device string) string {
	safe := strings.Map(func(letter rune) rune {
		if strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_", letter) {
			return letter
		}
		return '_'
	}, device)
	return filepath.Join(connectionsFolder(), "hyprtrixie-"+safe+".nmconnection")
}

func networkKey(line string) string {
	key := strings.Trim(strings.TrimPrefix(strings.TrimLeft(line, blanks), "wpa-psk"), blanks)
	return strings.TrimSuffix(strings.TrimPrefix(key, `"`), `"`)
}

func keyfileValue(text string) string {
	escaped := keyfileEscape.Replace(text)
	if strings.HasPrefix(escaped, " ") {
		escaped = `\s` + escaped[1:]
	}
	if strings.HasSuffix(escaped, " ") {
		escaped = escaped[:len(escaped)-1] + `\s`
	}
	return escaped
}

func ssidBytes(name string) string {
	var list strings.Builder
	for _, value := range []byte(name) {
		fmt.Fprintf(&list, "%d;", value)
	}
	return list.String()
}

func connectionID(network wifiNetwork) string {
	sum := sha1.Sum([]byte("hyprtrixie wifi\x00" + network.device + "\x00" + network.name))
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func keyfile(network wifiNetwork) string {
	text := "[connection]\nid=" + keyfileValue(network.name) + "\nuuid=" + connectionID(network) + "\ntype=wifi\ninterface-name=" + keyfileValue(network.device) +
		"\n\n[wifi]\nmode=infrastructure\nssid=" + ssidBytes(network.name) + "\n"
	if network.key != "" {
		text += "\n[wifi-security]\nkey-mgmt=wpa-psk\npsk=" + keyfileValue(network.key) + "\n"
	}
	return text + "\n[ipv4]\nmethod=auto\n\n[ipv6]\nmethod=auto\naddr-gen-mode=stable-privacy\n"
}

func fieldsOf(line string) []string {
	fields := strings.FieldsFunc(line, func(letter rune) bool { return strings.ContainsRune(blanks, letter) })
	return append(fields, "", "", "", "")
}

func ifaceReason(fields []string) string {
	family, method := fields[2], fields[3]
	if fields[1] == "lo" || family == "inet" && method == "dhcp" || family == "inet6" && (method == "auto" || method == "dhcp") {
		return ""
	}
	return fields[1] + " uses " + strings.TrimSpace(family+" "+method)
}

func stanzaStart(word string) bool {
	return slices.Contains(stanzaWords, word) || strings.HasPrefix(word, "allow-")
}

func unconvertible(lines []string) []string {
	var reasons []string
	device := ""
	for _, line := range lines {
		fields := fieldsOf(line)
		if fields[0] == "" || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch {
		case fields[0] == "iface":
			device = fields[1]
			reasons = append(reasons, ifaceReason(fields))
		case fields[0] == "mapping":
			device = ""
			reasons = append(reasons, "a mapping for "+fields[1])
		case stanzaStart(fields[0]):
			device = ""
		case device != "" && device != "lo" && !slices.Contains(handedOptions, fields[0]):
			reasons = append(reasons, device+" sets "+fields[0])
		}
	}
	return slices.DeleteFunc(reasons, func(reason string) bool { return reason == "" })
}
