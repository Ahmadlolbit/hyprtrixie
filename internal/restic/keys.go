package restic

import (
	"slices"
	"strings"
)

type Prompt struct {
	Name  string
	Label string
}

var allowedKeys = []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "B2_ACCOUNT_ID", "B2_ACCOUNT_KEY"}

func Prompts(location string) []Prompt {
	switch {
	case strings.HasPrefix(location, "s3:"):
		return []Prompt{{"AWS_ACCESS_KEY_ID", "Access key ID"}, {"AWS_SECRET_ACCESS_KEY", "Secret access key"}}
	case strings.HasPrefix(location, "b2:"):
		return []Prompt{{"B2_ACCOUNT_ID", "Account ID"}, {"B2_ACCOUNT_KEY", "Application key"}}
	}
	return nil
}

func Keys(text string) [][2]string {
	var found [][2]string
	for _, line := range strings.Split(text, "\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok && slices.Contains(allowedKeys, name) {
			found = append(found, [2]string{name, value})
		}
	}
	return found
}

func HasKeys(text, location string) bool {
	prompts := Prompts(location)
	return len(prompts) == 0 || strings.Contains("\n"+text, "\n"+prompts[0].Name+"=")
}
