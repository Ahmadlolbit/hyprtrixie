package editors

import (
	"encoding/json"
	"strings"
)

const Prefix = "Hyprtrixie "

type member struct {
	name       string
	value      string
	isString   bool
	valueStart int
	valueEnd   int
}

type scanner struct {
	text string
	at   int
}

func quoted(text string) string {
	data, _ := json.Marshal(text)
	return string(data)
}

func Ours(name string) bool {
	return strings.HasPrefix(name, Prefix)
}

func (scan *scanner) rest() string {
	return scan.text[scan.at:]
}

func (scan *scanner) skipComment() bool {
	switch {
	case strings.HasPrefix(scan.rest(), "//"):
		end := strings.IndexByte(scan.rest(), '\n')
		if end < 0 {
			end = len(scan.rest()) - 1
		}
		scan.at += end + 1
	case strings.HasPrefix(scan.rest(), "/*"):
		end := strings.Index(scan.rest()[2:], "*/")
		if end < 0 {
			scan.at = len(scan.text)
			return true
		}
		scan.at += end + 4
	default:
		return false
	}
	return true
}

func (scan *scanner) skip() {
	for scan.at < len(scan.text) {
		if strings.IndexByte(" \t\r\n", scan.text[scan.at]) >= 0 {
			scan.at++
		} else if !scan.skipComment() {
			return
		}
	}
}

func (scan *scanner) stringValue() (string, bool) {
	start := scan.at
	for scan.at++; scan.at < len(scan.text); scan.at++ {
		switch scan.text[scan.at] {
		case '\\':
			scan.at++
		case '"':
			scan.at++
			var value string
			return value, json.Unmarshal([]byte(scan.text[start:scan.at]), &value) == nil
		}
	}
	return "", false
}

func (scan *scanner) step(depth *int) (bool, bool) {
	switch char := scan.text[scan.at]; {
	case char == '"':
		_, ok := scan.stringValue()
		return false, ok
	case char == '{' || char == '[':
		*depth++
	case char == '}' || char == ']':
		if *depth == 0 {
			return true, true
		}
		*depth--
	case char == ',' && *depth == 0:
		return true, true
	}
	scan.at++
	return false, true
}

func (scan *scanner) value() bool {
	depth := 0
	for scan.skip(); scan.at < len(scan.text); scan.skip() {
		done, ok := scan.step(&depth)
		if done || !ok {
			return ok
		}
	}
	return depth == 0
}

func (scan *scanner) member() (member, bool) {
	name, ok := scan.stringValue()
	scan.skip()
	if !ok || scan.at >= len(scan.text) || scan.text[scan.at] != ':' {
		return member{}, false
	}
	scan.at++
	scan.skip()
	item := member{name: name, valueStart: scan.at}
	if scan.at < len(scan.text) && scan.text[scan.at] == '"' {
		item.value, item.isString = scan.stringValue()
	} else if !scan.value() {
		return member{}, false
	}
	item.valueEnd = scan.at
	return item, true
}

func members(text string) (int, []member, bool) {
	scan := &scanner{text: text}
	scan.skip()
	if scan.at >= len(text) || text[scan.at] != '{' {
		return 0, nil, false
	}
	open := scan.at
	var found []member
	for scan.at++; ; {
		scan.skip()
		switch {
		case scan.at >= len(text) || text[scan.at] != '"' && text[scan.at] != ',' && text[scan.at] != '}':
			return open, nil, false
		case text[scan.at] == '}':
			return open, found, true
		case text[scan.at] == ',':
			scan.at++
			continue
		}
		item, ok := scan.member()
		if !ok {
			return open, nil, false
		}
		found = append(found, item)
	}
}

func SetTheme(text, key, value string) (string, bool) {
	pair := quoted(key) + ": " + quoted(value)
	if strings.TrimSpace(text) == "" {
		return "{\n    " + pair + "\n}\n", true
	}
	open, found, ok := members(text)
	if !ok {
		return text, false
	}
	last := -1
	for index, item := range found {
		if item.name == key {
			last = index
		}
	}
	if last < 0 {
		end := ","
		if len(found) == 0 {
			end = "\n"
		}
		return text[:open+1] + "\n    " + pair + end + text[open+1:], true
	}
	item := found[last]
	if !item.isString || item.value == value || item.value != "" && !Ours(item.value) {
		return text, false
	}
	return text[:item.valueStart] + quoted(value) + text[item.valueEnd:], true
}
