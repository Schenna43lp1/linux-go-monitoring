//go:build linux

package main

import (
"context"
"fmt"
"strings"
"time"

"fyne.io/fyne/v2"
"fyne.io/fyne/v2/app"
"fyne.io/fyne/v2/container"
"fyne.io/fyne/v2/theme"
"fyne.io/fyne/v2/widget"
)

func main() {
a := app.New()
a.Settings().SetTheme(theme.DarkTheme())
w := a.NewWindow("Linux Monitor")
w.Resize(cfg.WindowSize)

cpuCard     := newDashCard("🖥  CPU",     colorCPU,     true,  false)
ramCard     := newDashCard("🧠  RAM",     colorRAM,     true,  false)
diskCard    := newDashCard("💾  DISK",    colorDisk,    true,  false)
netCard     := newDashCard("🌐  NETWORK", colorNetDown, false, true)
netDownCard := newDashCard("↓  Download", colorNetDown, false, true)
netUpCard   := newDashCard("↑  Upload",   colorNetUp,   false, true)

gpuUtilCard := newDashCard("🎮  GPU Utilization", colorGPU,  true,  false)
gpuVRAMCard := newDashCard("💠  VRAM",            colorVRAM, true,  false)
gpuNameLabel := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

collector               := newCollector()
sysInfoData             := collector.SystemInfo()
systemTab, uptimeValLabel := buildSystemTab(sysInfoData)

tabs := container.NewAppTabs(
buildOverviewTab(cpuCard, ramCard, diskCard, netCard),
buildNetworkTab(netDownCard, netUpCard),
systemTab,
		buildGPUTab(gpuUtilCard, gpuVRAMCard, gpuNameLabel),
	)

uptimeLabel := widget.NewLabel("")
darkMode := true
themeBtn := widget.NewButton("☀️ Light", nil)
themeBtn.OnTapped = func() {
if darkMode {
a.Settings().SetTheme(theme.LightTheme())
themeBtn.SetText("🌙 Dark")
} else {
a.Settings().SetTheme(theme.DarkTheme())
themeBtn.SetText("☀️ Light")
}
darkMode = !darkMode
}
header := container.NewBorder(nil, nil,
widget.NewLabelWithStyle("🖥  Linux Monitor", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
container.NewHBox(uptimeLabel, themeBtn),
)

alertLabel  := widget.NewLabel("")
footerLabel := widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true})

w.SetContent(container.NewBorder(
container.NewVBox(header, widget.NewSeparator(), alertLabel, widget.NewSeparator()),
container.NewVBox(widget.NewSeparator(), footerLabel),
nil, nil,
tabs,
))

ctx, cancel := context.WithCancel(context.Background())
w.SetOnClosed(cancel)

go RunLoop(ctx, collector, NewHistories(cfg.HistorySize), func(m Metrics, h *Histories) {
uptimeLabel.SetText("  |  Uptime: " + formatUptime(m.Uptime))
uptimeValLabel.SetText(formatUptime(m.Uptime))
footerLabel.SetText("Last updated: " + time.Now().Format("15:04:05"))

var alerts []string
if m.CPUPercent >= thresholdCrit {
alerts = append(alerts, fmt.Sprintf("CPU %.0f%%", m.CPUPercent))
}
if m.RAMPercent >= thresholdCrit {
alerts = append(alerts, fmt.Sprintf("RAM %.0f%%", m.RAMPercent))
}
if m.DiskPercent >= thresholdCrit {
alerts = append(alerts, fmt.Sprintf("Disk %.0f%%", m.DiskPercent))
}
if len(alerts) > 0 {
alertLabel.SetText("⚠️  Critical: " + strings.Join(alerts, " · "))
} else {
alertLabel.SetText("")
}

cpuCard.Status.SetText(statusDot(m.CPUPercent))
cpuCard.Value.SetText(fmt.Sprintf("%.1f%%", m.CPUPercent))
if m.HasTemp {
cpuCard.Sub.SetText(fmt.Sprintf("🌡  %.1f °C", m.CPUTemp))
}
cpuCard.Bar.SetValue(m.CPUPercent / 100)
cpuCard.Graph.Update(h.CPU.Values())

ramCard.Status.SetText(statusDot(m.RAMPercent))
ramCard.Value.SetText(fmt.Sprintf("%.1f%%", m.RAMPercent))
ramCard.Sub.SetText(fmt.Sprintf("%.1f / %.1f GB", m.RAMUsed/1e9, m.RAMTotal/1e9))
ramCard.Bar.SetValue(m.RAMPercent / 100)
ramCard.Graph.Update(h.RAM.Values())

diskCard.Status.SetText(statusDot(m.DiskPercent))
diskCard.Value.SetText(fmt.Sprintf("%.1f%%", m.DiskPercent))
diskCard.Sub.SetText(fmt.Sprintf("%.1f / %.1f GB", m.DiskUsed/1e9, m.DiskTotal/1e9))
diskCard.Bar.SetValue(m.DiskPercent / 100)
diskCard.Graph.Update(h.Disk.Values())

netCard.Value.SetText(fmt.Sprintf("↓  %s", formatSpeed(m.DownloadBps)))
netCard.Sub.SetText(fmt.Sprintf("↑  %s", formatSpeed(m.UploadBps)))
netCard.Graph.Update(h.NetDown.Values())

netDownCard.Value.SetText(formatSpeed(m.DownloadBps))
netDownCard.Graph.Update(h.NetDown.Values())

netUpCard.Value.SetText(formatSpeed(m.UploadBps))
netUpCard.Graph.Update(h.NetUp.Values())
})

w.ShowAndRun()
}
