package disks

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type slot struct {
	Start int64 `json:"start"`
	Size  int64 `json:"size"`
}

type Table struct {
	Label  string `json:"label"`
	First  int64  `json:"firstlba"`
	Last   int64  `json:"lastlba"`
	Sector int64  `json:"sectorsize"`
	Slots  []slot `json:"partitions"`
}

type Gap struct {
	Start int64
	Size  int64
}

func ReadTable(dump string) (Table, error) {
	var found struct {
		Table Table `json:"partitiontable"`
	}
	if err := json.Unmarshal([]byte(dump), &found); err != nil {
		return Table{}, err
	}
	if found.Table.Sector == 0 {
		found.Table.Sector = 512
	}
	return found.Table, nil
}

func alignUp(value, unit int64) int64 {
	return (value + unit - 1) / unit * unit
}

func (table Table) gap(from, to int64) (Gap, bool) {
	start := alignUp(from*table.Sector, MiB)
	end := (to + 1) * table.Sector / MiB * MiB
	return Gap{start, end - start}, end-start >= MiB
}

func (table Table) Gaps() []Gap {
	slots := slices.Clone(table.Slots)
	slices.SortFunc(slots, func(a, b slot) int { return cmp.Compare(a.Start, b.Start) })
	var gaps []Gap
	next := table.First
	for _, taken := range slots {
		if gap, ok := table.gap(next, taken.Start-1); ok {
			gaps = append(gaps, gap)
		}
		next = max(next, taken.Start+taken.Size)
	}
	if gap, ok := table.gap(next, table.Last); ok {
		gaps = append(gaps, gap)
	}
	return gaps
}

func (table Table) Largest() Gap {
	var best Gap
	for _, gap := range table.Gaps() {
		if gap.Size > best.Size {
			best = gap
		}
	}
	return best
}

func Size(bytes int64) string {
	if bytes >= 1000*1000*1000*1000 {
		return strings.Replace(fmt.Sprintf("%.1f TB", float64(bytes)/1e12), ".0 TB", " TB", 1)
	}
	return fmt.Sprintf("%d GB", (bytes+500*1000*1000)/(1000*1000*1000))
}
