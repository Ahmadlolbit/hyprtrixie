package tui

func (flow *wizard) startStage(machine []string) stage {
	return &page{wizard: flow, title: "Start", rows: func() []row {
		rows := []row{{label: "Debian 13 with Hyprland, set up properly", heading: true}, {info: true}}
		for _, line := range machine {
			rows = append(rows, row{label: line, info: true})
		}
		return rows
	}}
}
