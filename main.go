package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

func main() {
	a := app.New()
	w := a.NewWindow("Linux Monitor")
	w.Resize(fyne.NewSize(420, 260))

	cpuLabel := widget.NewLabel("CPU: lädt...")
	ramLabel := widget.NewLabel("RAM: lädt...")
	diskLabel := widget.NewLabel("Disk: lädt...")

	cpuBar := widget.NewProgressBar()
	ramBar := widget.NewProgressBar()
	diskBar := widget.NewProgressBar()

	content := container.NewVBox(
		widget.NewLabel("Systemübersicht"),
		cpuLabel,
		cpuBar,
		ramLabel,
		ramBar,
		diskLabel,
		diskBar,
	)

	w.SetContent(content)

	go func() {
		for {
			cpuPercent, _ := cpu.Percent(0, false)
			vmem, _ := mem.VirtualMemory()
			diskStat, _ := disk.Usage("/")

			if len(cpuPercent) > 0 {
				c := cpuPercent[0]
				cpuLabel.SetText(fmt.Sprintf("CPU: %.2f%%", c))
				cpuBar.SetValue(c / 100)
			}

			ramLabel.SetText(fmt.Sprintf(
				"RAM: %.2f%% (%.1f / %.1f GB)",
				vmem.UsedPercent,
				float64(vmem.Used)/1024/1024/1024,
				float64(vmem.Total)/1024/1024/1024,
			))
			ramBar.SetValue(vmem.UsedPercent / 100)

			diskLabel.SetText(fmt.Sprintf(
				"Disk: %.2f%% (%.1f / %.1f GB)",
				diskStat.UsedPercent,
				float64(diskStat.Used)/1024/1024/1024,
				float64(diskStat.Total)/1024/1024/1024,
			))
			diskBar.SetValue(diskStat.UsedPercent / 100)

			time.Sleep(2 * time.Second)
		}
	}()

	w.ShowAndRun()
}
