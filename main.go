//go:build linux

package main

// This is a simple Linux system monitor application built using the Fyne GUI toolkit. It displays CPU, RAM, and Disk usage in a window, updating every 2 seconds.
// The application uses the gopsutil library to retrieve system statistics and Fyne to create the user interface. It also handles graceful shutdown when the window is closed.
// Note: Ensure you have the necessary permissions to access system stats and that you have the gopsutil library installed in your Go environment.
// To run this application, save the code in a file named main.go, and execute `go run main.go` in your terminal. You should see a window displaying the current CPU, RAM, and Disk usage of your Linux system.
// Make sure to have the Fyne library installed as well, which you can do using `go get fyne.io/fyne/v2`.
// This code is intended for Linux systems and may not work correctly on other operating systems due to differences in how system statistics are accessed and displayed.
import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// CPUHistory stores historical CPU load values
type CPUHistory struct {
	mu      sync.Mutex
	values  []float64
	maxSize int
}

// NewCPUHistory creates a new CPU history tracker
func NewCPUHistory(maxSize int) *CPUHistory {
	return &CPUHistory{
		values:  make([]float64, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a new CPU value to history
func (h *CPUHistory) Add(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, value)
	if len(h.values) > h.maxSize {
		h.values = h.values[1:]
	}
}

// GetSparkline returns a sparkline representation of CPU history
func (h *CPUHistory) GetSparkline() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.values) == 0 {
		return ""
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	result := ""
	for _, val := range h.values {
		idx := int(val / 100 * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		result += string(blocks[idx])
	}
	return result
}

// GetValues returns a copy of the current values
func (h *CPUHistory) GetValues() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	values := make([]float64, len(h.values))
	copy(values, h.values)
	return values
}

func statCard(title string, content fyne.CanvasObject, bar *widget.ProgressBar) *fyne.Container {
	return container.NewVBox(
		widget.NewLabel(title),
		content,
		bar,
		widget.NewSeparator(),
	)
}

// containerWithHistory creates a container showing both current value and history
func containerWithHistory(current *widget.Label, history *widget.Label) *fyne.Container {
	return container.NewVBox(
		current,
		history,
	)
}

// main initializes the Fyne application, creates a window, and sets up the UI to display CPU, RAM, and Disk usage. It also starts a goroutine to periodically update these stats every 2 seconds until the window is closed.
func main() {
	a := app.New()
	w := a.NewWindow("Linux Monitor")
	w.Resize(fyne.NewSize(520, 320))

	title := widget.NewLabel("Linux Monitor")

	cpuLabel := widget.NewLabel("lädt...")
	cpuHistoryLabel := widget.NewLabel("History: -")
	ramLabel := widget.NewLabel("lädt...")
	diskLabel := widget.NewLabel("lädt...")

	cpuBar := widget.NewProgressBar()
	ramBar := widget.NewProgressBar()
	diskBar := widget.NewProgressBar()

	cpuHistory := NewCPUHistory(20)

	grid := container.NewGridWithColumns(1,
		statCard("CPU", containerWithHistory(cpuLabel, cpuHistoryLabel), cpuBar),
		statCard("RAM", ramLabel, ramBar),
		statCard("DISK", diskLabel, diskBar),
	)

	w.SetContent(container.NewVBox(
		title,
		widget.NewSeparator(),
		grid,
	))

	quit := make(chan struct{})

	w.SetOnClosed(func() {
		close(quit)
	})

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				cpuPercent, _ := cpu.Percent(0, false)
				vmem, _ := mem.VirtualMemory()
				diskStat, _ := disk.Usage("/")

				fyne.Do(func() {
					if len(cpuPercent) > 0 {
						cpuHistory.Add(cpuPercent[0])
						cpuBar.SetValue(cpuPercent[0] / 100)
						cpuLabel.SetText(fmt.Sprintf("%.2f%%", cpuPercent[0]))
						cpuHistoryLabel.SetText("History: " + cpuHistory.GetSparkline())
					}

					ramBar.SetValue(vmem.UsedPercent / 100)
					ramLabel.SetText(fmt.Sprintf("%.2f%%  (%.1f / %.1f GB)",
						vmem.UsedPercent,
						float64(vmem.Used)/1024/1024/1024,
						float64(vmem.Total)/1024/1024/1024,
					))

					diskBar.SetValue(diskStat.UsedPercent / 100)
					diskLabel.SetText(fmt.Sprintf("%.2f%%  (%.1f / %.1f GB)",
						diskStat.UsedPercent,
						float64(diskStat.Used)/1024/1024/1024,
						float64(diskStat.Total)/1024/1024/1024,
					))
				})
			}
		}
	}()

	w.ShowAndRun()
}
