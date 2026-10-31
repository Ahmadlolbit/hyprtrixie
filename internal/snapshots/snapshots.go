package snapshots

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

const (
	dateLayout  = "2006-01-02 15:04:05"
	shownLayout = "Mon 2 Jan 15:04"
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
	return found
}

func All(limit time.Duration, options ...string) ([]Snapshot, error) {
	args := append(options, "--csvout", "--no-headers", "--separator", "\t", "-c", "root", "list", "--columns", "number,type,date,description")
	output, err := run.CaptureQuiet(limit, "snapper", args...)
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
	folders := make([]string, 0, len(counts))
	for folder := range counts {
		folders = append(folders, folder)
	}
	slices.SortFunc(folders, func(first, second string) int {
		return cmp.Or(cmp.Compare(counts[second], counts[first]), cmp.Compare(first, second))
	})
	return folders[:min(maxFolders, len(folders))]
}

func Changes(status string) string {
	kinds := map[byte]int{}
	counts := map[string]int{}
	for _, line := range run.Lines(status) {
		if fields := strings.Fields(line); len(fields) >= 2 {
			kinds[fields[0][0]]++
			counts[folderOf(fields[1])]++
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
	output, _ := run.CaptureQuiet(run.Quick, "df", "-P", "/")
	lines := run.Lines(output)
	if len(lines) < 2 {
		return 0, false
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 0, false
	}
	use, err := strconv.Atoi(strings.TrimSuffix(fields[4], "%"))
	return use, err == nil
}

func KernelAdded(status, kernel string) bool {
	return strings.Contains("\n"+status+"\n", "\n+..... /usr/lib/modules/"+kernel+"\n")
}
