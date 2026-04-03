//go:build linux

package main

import (
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
)

// MetricHistory stores a rolling window of metric values.
type MetricHistory struct {
	mu      sync.Mutex
	values  []float64
	maxSize int
}

func NewMetricHistory(maxSize int) *MetricHistory {
	return &MetricHistory{
		values:  make([]float64, 0, maxSize),
		maxSize: maxSize,
	}
}

func (h *MetricHistory) Add(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, value)
	if len(h.values) > h.maxSize {
		h.values = h.values[1:]
	}
}

func (h *MetricHistory) GetValues() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]float64, len(h.values))
	copy(out, h.values)
	return out
}

// GraphWidget draws a line graph.
// When autoScale is true the Y-axis adapts to the current max value in the window;
// otherwise values are assumed to be in the range 0–100.
type GraphWidget struct {
	widget.BaseWidget
	values    []float64
	maxSize   int
	lineColor color.Color
	autoScale bool
}

func NewGraphWidget(maxSize int, c color.Color) *GraphWidget {
	g := &GraphWidget{maxSize: maxSize, lineColor: c}
	g.ExtendBaseWidget(g)
	return g
}

func NewAutoScaleGraphWidget(maxSize int, c color.Color) *GraphWidget {
	g := &GraphWidget{maxSize: maxSize, lineColor: c, autoScale: true}
	g.ExtendBaseWidget(g)
	return g
}

// Update replaces the displayed values and triggers a repaint (must be called on the main goroutine).
func (g *GraphWidget) Update(values []float64) {
	g.values = make([]float64, len(values))
	copy(g.values, values)
	g.Refresh()
}

func (g *GraphWidget) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.NRGBA{R: 18, G: 18, B: 18, A: 255})

	gridColor := color.NRGBA{R: 55, G: 55, B: 55, A: 255}
	midColor := color.NRGBA{R: 75, G: 75, B: 75, A: 255}
	grid25 := canvas.NewLine(gridColor)
	grid50 := canvas.NewLine(midColor)
	grid75 := canvas.NewLine(gridColor)

	lines := make([]*canvas.Line, g.maxSize-1)
	for i := range lines {
		l := canvas.NewLine(g.lineColor)
		l.StrokeWidth = 1.5
		l.Hidden = true
		lines[i] = l
	}

	objects := make([]fyne.CanvasObject, 0, 4+len(lines))
	objects = append(objects, bg, grid25, grid50, grid75)
	for _, l := range lines {
		objects = append(objects, l)
	}

	return &graphRenderer{
		graph:   g,
		bg:      bg,
		grid25:  grid25,
		grid50:  grid50,
		grid75:  grid75,
		lines:   lines,
		objects: objects,
	}
}

type graphRenderer struct {
	graph   *GraphWidget
	bg      *canvas.Rectangle
	grid25  *canvas.Line
	grid50  *canvas.Line
	grid75  *canvas.Line
	lines   []*canvas.Line
	objects []fyne.CanvasObject
	size    fyne.Size
}

func (r *graphRenderer) Layout(size fyne.Size) {
	r.size = size
	r.bg.Resize(size)
	r.Refresh()
}

func (r *graphRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 80) }

func (r *graphRenderer) Refresh() {
	w, h := r.size.Width, r.size.Height

	values := r.graph.values
	maxVal := 100.0
	if r.graph.autoScale {
		maxVal = 1.0
		for _, v := range values {
			if v > maxVal {
				maxVal = v
			}
		}
	}

	setGrid := func(l *canvas.Line, pct float32) {
		y := h * (1 - pct)
		l.Position1 = fyne.NewPos(0, y)
		l.Position2 = fyne.NewPos(w, y)
		l.Refresh()
	}
	setGrid(r.grid25, 0.25)
	setGrid(r.grid50, 0.50)
	setGrid(r.grid75, 0.75)

	n := len(values)
	maxPts := r.graph.maxSize

	for i, l := range r.lines {
		if i >= n-1 {
			l.Hidden = true
			l.Refresh()
			continue
		}
		l.Hidden = false
		x1 := float32(i) / float32(maxPts-1) * w
		y1 := h - float32(values[i]/maxVal)*h
		x2 := float32(i+1) / float32(maxPts-1) * w
		y2 := h - float32(values[i+1]/maxVal)*h
		l.Position1 = fyne.NewPos(x1, y1)
		l.Position2 = fyne.NewPos(x2, y2)
		l.Refresh()
	}
	r.bg.Refresh()
}

func (r *graphRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *graphRenderer) Destroy()                     {}

func statCard(title string, valueLabel *widget.Label, bar *widget.ProgressBar, graph *GraphWidget) *fyne.Container {
	return container.NewVBox(
		widget.NewLabel(title),
		valueLabel,
		bar,
		graph,
		widget.NewSeparator(),
	)
}

func statCardNoBar(title string, labels []fyne.CanvasObject, graph *GraphWidget) *fyne.Container {
	items := []fyne.CanvasObject{widget.NewLabel(title)}
	items = append(items, labels...)
	items = append(items, graph, widget.NewSeparator())
	return container.NewVBox(items...)
}

func formatSpeed(bps float64) string {
	switch {
	case bps >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", bps/1024/1024)
	case bps >= 1024:
		return fmt.Sprintf("%.1f KB/s", bps/1024)
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}

func formatUptime(secs uint64) string {
	days := secs / 86400
	hours := (secs % 86400) / 3600
	minutes := (secs % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// getCPUTemp returns the CPU temperature in °C.
// It prefers labelled CPU/core sensors and falls back to the highest reading.
func getCPUTemp() (float64, bool) {
	temps, err := host.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return 0, false
	}
	for _, t := range temps {
		key := strings.ToLower(t.SensorKey)
		if strings.Contains(key, "cpu") || strings.Contains(key, "core") ||
			strings.Contains(key, "k10temp") || strings.Contains(key, "coretemp") ||
			strings.Contains(key, "tctl") {
			return t.Temperature, true
		}
	}
	max := temps[0].Temperature
	for _, t := range temps[1:] {
		if t.Temperature > max {
			max = t.Temperature
		}
	}
	return max, true
}

func main() {
	a := app.New()
	w := a.NewWindow("Linux Monitor")
	w.Resize(fyne.NewSize(620, 780))

	uptimeLabel := widget.NewLabel("")
	header := container.NewHBox(widget.NewLabel("Linux Monitor"), uptimeLabel)

	cpuLabel := widget.NewLabel("lädt...")
	cpuTempLabel := widget.NewLabel("")
	ramLabel := widget.NewLabel("lädt...")
	diskLabel := widget.NewLabel("lädt...")
	netUpLabel := widget.NewLabel("↑  –")
	netDownLabel := widget.NewLabel("↓  –")

	cpuBar := widget.NewProgressBar()
	ramBar := widget.NewProgressBar()
	diskBar := widget.NewProgressBar()

	const histSize = 60
	cpuHistory := NewMetricHistory(histSize)
	ramHistory := NewMetricHistory(histSize)
	diskHistory := NewMetricHistory(histSize)
	netDownHistory := NewMetricHistory(histSize)

	cpuGraph := NewGraphWidget(histSize, color.NRGBA{R: 0, G: 210, B: 100, A: 255})
	ramGraph := NewGraphWidget(histSize, color.NRGBA{R: 30, G: 150, B: 255, A: 255})
	diskGraph := NewGraphWidget(histSize, color.NRGBA{R: 255, G: 150, B: 0, A: 255})
	netGraph := NewAutoScaleGraphWidget(histSize, color.NRGBA{R: 220, G: 80, B: 255, A: 255})

	cpuCard := container.NewVBox(
		widget.NewLabel("CPU"),
		container.NewHBox(cpuLabel, cpuTempLabel),
		cpuBar,
		cpuGraph,
		widget.NewSeparator(),
	)

	grid := container.NewGridWithColumns(1,
		cpuCard,
		statCard("RAM", ramLabel, ramBar, ramGraph),
		statCard("DISK", diskLabel, diskBar, diskGraph),
		statCardNoBar("NETWORK  (↓ Download)", []fyne.CanvasObject{netUpLabel, netDownLabel}, netGraph),
	)

	w.SetContent(container.NewVBox(
		header,
		widget.NewSeparator(),
		grid,
	))

	quit := make(chan struct{})
	w.SetOnClosed(func() { close(quit) })

	// Seed initial network counters
	var prevSent, prevRecv uint64
	var prevNetTime time.Time
	if ctrs, err := psnet.IOCounters(false); err == nil && len(ctrs) > 0 {
		prevSent = ctrs[0].BytesSent
		prevRecv = ctrs[0].BytesRecv
		prevNetTime = time.Now()
	}

	go func() {
		cpu.Percent(0, false) // warmup: initializes the baseline measurement
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
				uptime, _ := host.Uptime()
				cpuTemp, hasTemp := getCPUTemp()

				var uploadBps, downloadBps float64
				if ctrs, err := psnet.IOCounters(false); err == nil && len(ctrs) > 0 {
					now := time.Now()
					dt := now.Sub(prevNetTime).Seconds()
					if dt > 0 {
						uploadBps = float64(ctrs[0].BytesSent-prevSent) / dt
						downloadBps = float64(ctrs[0].BytesRecv-prevRecv) / dt
					}
					prevSent = ctrs[0].BytesSent
					prevRecv = ctrs[0].BytesRecv
					prevNetTime = now
				}

				if len(cpuPercent) > 0 {
					cpuHistory.Add(cpuPercent[0])
				}
				ramHistory.Add(vmem.UsedPercent)
				diskHistory.Add(diskStat.UsedPercent)
				netDownHistory.Add(downloadBps)

				fyne.Do(func() {
					uptimeLabel.SetText("  |  Uptime: " + formatUptime(uptime))

					if len(cpuPercent) > 0 {
						cpuBar.SetValue(cpuPercent[0] / 100)
						cpuLabel.SetText(fmt.Sprintf("%.2f%%", cpuPercent[0]))
						cpuGraph.Update(cpuHistory.GetValues())
					}
					if hasTemp {
						cpuTempLabel.SetText(fmt.Sprintf("   🌡 %.1f °C", cpuTemp))
					}

					ramBar.SetValue(vmem.UsedPercent / 100)
					ramLabel.SetText(fmt.Sprintf("%.2f%%  (%.1f / %.1f GB)",
						vmem.UsedPercent,
						float64(vmem.Used)/1024/1024/1024,
						float64(vmem.Total)/1024/1024/1024,
					))
					ramGraph.Update(ramHistory.GetValues())

					diskBar.SetValue(diskStat.UsedPercent / 100)
					diskLabel.SetText(fmt.Sprintf("%.2f%%  (%.1f / %.1f GB)",
						diskStat.UsedPercent,
						float64(diskStat.Used)/1024/1024/1024,
						float64(diskStat.Total)/1024/1024/1024,
					))
					diskGraph.Update(diskHistory.GetValues())

					netUpLabel.SetText("↑  " + formatSpeed(uploadBps))
					netDownLabel.SetText("↓  " + formatSpeed(downloadBps))
					netGraph.Update(netDownHistory.GetValues())
				})
			}
		}
	}()

	w.ShowAndRun()
}

