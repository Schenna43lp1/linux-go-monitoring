//go:build linux

package main

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// MetricHistory stores a rolling window of metric values (0–100).
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

// GraphWidget is a custom Fyne widget that draws a line graph for a 0–100 metric.
type GraphWidget struct {
	widget.BaseWidget
	values    []float64
	maxSize   int
	lineColor color.Color
}

func NewGraphWidget(maxSize int, c color.Color) *GraphWidget {
	g := &GraphWidget{maxSize: maxSize, lineColor: c}
	g.ExtendBaseWidget(g)
	return g
}

// Update replaces the displayed values and triggers a repaint (call on main goroutine).
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

	setGrid := func(l *canvas.Line, pct float32) {
		y := h * (1 - pct)
		l.Position1 = fyne.NewPos(0, y)
		l.Position2 = fyne.NewPos(w, y)
		l.Refresh()
	}
	setGrid(r.grid25, 0.25)
	setGrid(r.grid50, 0.50)
	setGrid(r.grid75, 0.75)

	values := r.graph.values
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
		y1 := h - float32(values[i])/100*h
		x2 := float32(i+1) / float32(maxPts-1) * w
		y2 := h - float32(values[i+1])/100*h
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

func main() {
	a := app.New()
	w := a.NewWindow("Linux Monitor")
	w.Resize(fyne.NewSize(600, 560))

	title := widget.NewLabel("Linux Monitor")

	cpuLabel := widget.NewLabel("lädt...")
	ramLabel := widget.NewLabel("lädt...")
	diskLabel := widget.NewLabel("lädt...")

	cpuBar := widget.NewProgressBar()
	ramBar := widget.NewProgressBar()
	diskBar := widget.NewProgressBar()

	const histSize = 60
	cpuHistory := NewMetricHistory(histSize)
	ramHistory := NewMetricHistory(histSize)
	diskHistory := NewMetricHistory(histSize)

	cpuGraph := NewGraphWidget(histSize, color.NRGBA{R: 0, G: 210, B: 100, A: 255})
	ramGraph := NewGraphWidget(histSize, color.NRGBA{R: 30, G: 150, B: 255, A: 255})
	diskGraph := NewGraphWidget(histSize, color.NRGBA{R: 255, G: 150, B: 0, A: 255})

	grid := container.NewGridWithColumns(1,
		statCard("CPU", cpuLabel, cpuBar, cpuGraph),
		statCard("RAM", ramLabel, ramBar, ramGraph),
		statCard("DISK", diskLabel, diskBar, diskGraph),
	)

	w.SetContent(container.NewVBox(
		title,
		widget.NewSeparator(),
		grid,
	))

	quit := make(chan struct{})
	w.SetOnClosed(func() { close(quit) })

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

				if len(cpuPercent) > 0 {
					cpuHistory.Add(cpuPercent[0])
				}
				ramHistory.Add(vmem.UsedPercent)
				diskHistory.Add(diskStat.UsedPercent)

				fyne.Do(func() {
					if len(cpuPercent) > 0 {
						cpuBar.SetValue(cpuPercent[0] / 100)
						cpuLabel.SetText(fmt.Sprintf("%.2f%%", cpuPercent[0]))
						cpuGraph.Update(cpuHistory.GetValues())
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
				})
			}
		}
	}()

	w.ShowAndRun()
}
