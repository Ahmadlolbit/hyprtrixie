package words

import (
	"cmp"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
)

type pattern struct {
	english    *regexp.Regexp
	translated string
}

var (
	table     = sync.OnceValue(func() map[string]string { return Catalogue(Language()) })
	patterns  = sync.OnceValue(func() []pattern { return patternsOf(table()) })
	verb      = regexp.MustCompile(`%[0-9]*[sdvq]`)
	escaper   = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\t", "\\t")
	unescaper = strings.NewReplacer("\\\\", "\\", "\\n", "\n", "\\t", "\t")
)

func bare(value string) string {
	value, _, _ = strings.Cut(value, ":")
	value, _, _ = strings.Cut(value, ".")
	value, _, _ = strings.Cut(value, "@")
	return value
}

func Language() string {
	if own := bare(os.Getenv("HYPRTRIXIE_LANGUAGE")); own != "" {
		return own
	}
	locale := bare(cmp.Or(os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG")))
	if locale == "" || locale == "C" || locale == "POSIX" {
		return ""
	}
	return cmp.Or(bare(os.Getenv("LANGUAGE")), locale)
}

func escape(text string) string {
	return escaper.Replace(text)
}

func sameVerbs(english, translated string) bool {
	return slices.Equal(verb.FindAllString(english, -1), verb.FindAllString(translated, -1))
}

func Read(text string) map[string]string {
	found := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		english, translated, ok := strings.Cut(line, "\t")
		english, translated = unescaper.Replace(english), unescaper.Replace(translated)
		if ok && english != "" && !strings.HasPrefix(english, "#") && strings.TrimSpace(translated) != "" && sameVerbs(english, translated) {
			found[english] = translated
		}
	}
	return found
}

func Catalogue(language string) map[string]string {
	short, _, _ := strings.Cut(language, "_")
	for _, name := range []string{language, short} {
		if name == "" || strings.ContainsAny(name, "/\\") {
			continue
		}
		if path := filepath.Join(env.Manifests(), "locale", name+".tsv"); fsutil.IsFile(path) {
			return Read(fsutil.Optional(path))
		}
	}
	return map[string]string{}
}

func patternsOf(found map[string]string) []pattern {
	var all []pattern
	for english, translated := range found {
		pieces := verb.Split(english, -1)
		if len(pieces) < 2 {
			continue
		}
		for index, piece := range pieces {
			pieces[index] = regexp.QuoteMeta(piece)
		}
		all = append(all, pattern{regexp.MustCompile("^" + strings.Join(pieces, "(.+?)") + "$"), translated})
	}
	slices.SortFunc(all, func(a, b pattern) int {
		return cmp.Or(cmp.Compare(len(b.english.String()), len(a.english.String())), strings.Compare(a.english.String(), b.english.String()))
	})
	return all
}

func (rule pattern) apply(text string) (string, bool) {
	values := rule.english.FindStringSubmatch(text)
	if values == nil {
		return "", false
	}
	pieces := verb.Split(rule.translated, -1)
	var out strings.Builder
	for index, piece := range pieces {
		out.WriteString(piece)
		if index+1 < len(pieces) {
			out.WriteString(values[index+1])
		}
	}
	return out.String(), true
}

func T(english string) string {
	if translated, ok := table()[english]; ok {
		return translated
	}
	for _, rule := range patterns() {
		if translated, ok := rule.apply(english); ok {
			return translated
		}
	}
	return english
}
