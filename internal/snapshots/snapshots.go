package snapshots

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Ahmadlolbit/hyprtrixie/internal/fsutil"
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	dateLayout  = "2006-01-02 15:04:05"
	shownLayout = "Mon 2 Jan 2006 15:04"
	maxFolders  = 8
	keptNewest  = 3
)

type Snapshot struct {
	Number      string
	Kind        string
	Date        string
	Description string
}

func IsNumber(text string) bool {
	return text != "" && text[0] != '0' && strings.Trim(text, "0123456789") == ""
}

func parse(output string) []Snapshot {
	var found []Snapshot
	for _, line := range run.Lines(output) {
		fields := append(strings.SplitN(line, "\t", 4), "", "", "")
		if IsNumber(fields[0]) {
			found = append(found, Snapshot{fields[0], fields[1], fields[2], fields[3]})
		}
	}
	slices.SortStableFunc(found, func(first, second Snapshot) int {
		return cmp.Or(cmp.Compare(len(first.Number), len(second.Number)), cmp.Compare(first.Number, second.Number))
	})
	return found
}

func All(limit time.Duration) ([]Snapshot, error) {
	args := []string{"--csvout", "--no-headers", "--separator", "\t", "-c", "root", "list", "--columns", "number,type,date,description"}
	if os.Geteuid() == 0 {
		args = append([]string{"--no-dbus"}, args...)
	}
	output, err := run.Capture(limit, "snapper", args...)
	return parse(output), err
}

func (snapshot Snapshot) reason() string {
	switch {
	case snapshot.Description == "timeline":
		return "Automatic"
	case snapshot.Description == "apt" && snapshot.Kind == "pre":
		return "Before packages changed"
	case snapshot.Description == "apt" && snapshot.Kind == "post":
		return "After packages changed"
	}
	return cmp.Or(snapshot.Description, "No description")
}

func (snapshot Snapshot) Label() string {
	when := snapshot.Date
	if at, err := time.ParseInLocation(dateLayout, snapshot.Date, time.Local); err == nil {
		when = at.Format(shownLayout)
	}
	return when + ", " + snapshot.reason()
}

func folderOf(path string) string {
	parts := append(strings.Split(path, "/"), "", "")
	if strings.Count(path, "/") > 2 {
		return "/" + parts[1] + "/" + parts[2]
	}
	return "/" + parts[1]
}

func biggest(counts map[string]int) []string {
	folders := slices.SortedFunc(maps.Keys(counts), func(first, second string) int {
		return cmp.Or(cmp.Compare(counts[second], counts[first]), cmp.Compare(first, second))
	})
	return folders[:min(maxFolders, len(folders))]
}

func printable(text string) string {
	return strings.Map(func(letter rune) rune {
		if unicode.IsControl(letter) {
			return -1
		}
		return letter
	}, text)
}

func Changes(status string) string {
	kinds := map[byte]int{}
	counts := map[string]int{}
	for _, line := range run.Lines(status) {
		if kind, path, found := strings.Cut(strings.TrimLeft(line, " "), " "); found {
			kinds[kind[0]]++
			counts[folderOf(printable(strings.TrimLeft(path, " ")))]++
		}
	}
	var text strings.Builder
	fmt.Fprintf(&text, "\nSince then: %d added, %d removed, %d changed\nMost of it is in:\n", kinds['+'], kinds['-'], kinds['c']+kinds['t'])
	for _, folder := range biggest(counts) {
		fmt.Fprintf(&text, "%7d %s\n", counts[folder], folder)
	}
	return text.String()
}

func Disposable(all []Snapshot) []string {
	var numbers []string
	for _, snapshot := range all[:max(len(all)-keptNewest, 0)] {
		if snapshot.Description == "timeline" || snapshot.Description == "apt" {
			numbers = append(numbers, snapshot.Number)
		}
	}
	return numbers
}

func DiskUse() (int, bool) {
	output, err := run.CaptureQuiet(run.Quick, "df", "-P", "/")
	found := fsutil.Filesystems(output)
	if err != nil || len(found) == 0 || !found[0].Known {
		return 0, false
	}
	return found[0].Use, true
}

func Count(number int, noun string) string {
	if number == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(number) + " " + noun + "s"
}

func KernelAdded(status, kernel string) bool {
	return strings.Contains("\n"+status+"\n", "\n+..... /usr/lib/modules/"+kernel+"\n")
}
