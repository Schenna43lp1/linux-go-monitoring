//go:build linux

// ui.go – baut alle Tabs (Overview, Network, System, GPU) und das Hauptfenster der Fyne-App.
package main

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// buildOverviewTab creates the Overview tab with CPU, RAM, Disk, Network cards.
func buildOverviewTab(cpu, ram, disk, net *dashCard) *container.TabItem {
return container.NewTabItem("Overview",
container.NewScroll(container.NewGridWithColumns(2,
cpu.Container,
ram.Container,
disk.Container,
net.Container,
)),
)
}

// buildNetworkTab creates the Network tab with separate download/upload cards.
func buildNetworkTab(down, up *dashCard) *container.TabItem {
return container.NewTabItem("Network",
container.NewScroll(container.NewVBox(
down.Container,
up.Container,
)),
)
}

// buildSystemTab creates the System tab with static info and live uptime.
// Returns the tab and the uptimeValLabel that needs to be updated each tick.
func buildSystemTab(info SystemInfo) (*container.TabItem, *widget.Label) {
uptimeValLabel := widget.NewLabel("–")
tab := container.NewTabItem("System",
container.NewScroll(container.NewPadded(container.NewVBox(
widget.NewLabelWithStyle("System Information", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
widget.NewSeparator(),
container.NewGridWithColumns(2,
widget.NewLabel("Hostname"), widget.NewLabel(info.Hostname),
widget.NewLabel("OS"),       widget.NewLabel(info.OS),
widget.NewLabel("Kernel"),   widget.NewLabel(info.Kernel),
widget.NewLabel("Uptime"),   uptimeValLabel,
),
))),
)
return tab, uptimeValLabel
}

// buildGPUTab creates the GPU tab with utilization and VRAM cards.
func buildGPUTab(util, vram *dashCard, nameLabel *widget.Label) *container.TabItem {
	return container.NewTabItem("GPU",
		container.NewScroll(container.NewVBox(
			container.NewPadded(nameLabel),
			util.Container,
			vram.Container,
		)),
	)
}

// buildProcessesTab creates the Processes tab with a live-updating table of top processes.
// Returns the tab and an update function to be called each tick.
func buildProcessesTab() (*container.TabItem, func([]ProcessInfo)) {
	headers := []string{"PID", "Name", "CPU %", "RAM %", "RAM MB"}
	colWidths := []float32{65, 200, 70, 70, 80}

	var mu sync.Mutex
	var rows []ProcessInfo

	table := widget.NewTable(
		func() (int, int) {
			mu.Lock()
			defer mu.Unlock()
			return len(rows) + 1, len(headers)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			mu.Lock()
			defer mu.Unlock()
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(headers[id.Col])
				return
			}
			label.TextStyle = fyne.TextStyle{}
			i := id.Row - 1
			if i >= len(rows) {
				label.SetText("")
				return
			}
			p := rows[i]
			switch id.Col {
			case 0:
				label.SetText(fmt.Sprintf("%d", p.PID))
			case 1:
				label.SetText(p.Name)
			case 2:
				label.SetText(fmt.Sprintf("%.1f", p.CPUPct))
			case 3:
				label.SetText(fmt.Sprintf("%.1f", p.RAMPct))
			case 4:
				label.SetText(fmt.Sprintf("%.0f", p.RAMMB))
			}
		},
	)
	for i, w := range colWidths {
		table.SetColumnWidth(i, w)
	}

	update := func(procs []ProcessInfo) {
		mu.Lock()
		rows = procs
		mu.Unlock()
		table.Refresh()
	}

	tab := container.NewTabItem("⚙ Processes",
		container.NewBorder(nil, nil, nil, nil, table),
	)
	return tab, update
}
