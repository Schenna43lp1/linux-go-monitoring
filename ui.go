//go:build linux

// ui.go – baut alle Tabs (Overview, Network, System, GPU) und das Hauptfenster der Fyne-App.
package main

import (
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
