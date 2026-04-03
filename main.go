//go:build linux

package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ── GraphWidget ───────────────────────────────────────────────────────────────

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

// ── UI Helpers ────────────────────────────────────────────────────────────────

// newCard builds a stat card with an optional progress bar (pass nil to omit it).
// Additional label rows are passed as variadic CanvasObjects.
func newCard(title string, bar *widget.ProgressBar, graph *GraphWidget, rows ...fyne.CanvasObject) *fyne.Container {
	items := []fyne.CanvasObject{
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	}
	items = append(items, rows...)
	if bar != nil {
		items = append(items, bar)
	}
	items = append(items, graph)
	return container.NewPadded(container.NewVBox(items...))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	a := app.New()
	w := a.NewWindow("Linux Monitor")
	w.Resize(cfg.WindowSize)

	// Labels
	uptimeLabel := widget.NewLabel("")
	cpuLabel := widget.NewLabel("lädt...")
	cpuTempLabel := widget.NewLabel("")
	ramLabel := widget.NewLabel("lädt...")
	diskLabel := widget.NewLabel("lädt...")
	netUpLabel := widget.NewLabel("↑  –")
	netDownLabel := widget.NewLabel("↓  –")

	// Progress bars
	cpuBar := widget.NewProgressBar()
	ramBar := widget.NewProgressBar()
	diskBar := widget.NewProgressBar()

	// Graphs
	cpuGraph := NewGraphWidget(cfg.HistorySize, color.NRGBA{R: 0, G: 210, B: 100, A: 255})
	ramGraph := NewGraphWidget(cfg.HistorySize, color.NRGBA{R: 30, G: 150, B: 255, A: 255})
	diskGraph := NewGraphWidget(cfg.HistorySize, color.NRGBA{R: 255, G: 150, B: 0, A: 255})
	netGraph := NewAutoScaleGraphWidget(cfg.HistorySize, color.NRGBA{R: 220, G: 80, B: 255, A: 255})

	// Layout
	header := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("🖥  Linux Monitor", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		uptimeLabel,
	)
	grid := container.NewGridWithColumns(2,
		newCard("CPU", cpuBar, cpuGraph, container.NewHBox(cpuLabel, cpuTempLabel)),
		newCard("RAM", ramBar, ramGraph, ramLabel),
		newCard("DISK", diskBar, diskGraph, diskLabel),
		newCard("NETWORK", nil, netGraph, netUpLabel, netDownLabel),
	)
	w.SetContent(container.NewPadded(container.NewVBox(header, widget.NewSeparator(), grid)))

	// Wire update loop
	quit := make(chan struct{})
	w.SetOnClosed(func() { close(quit) })

	go RunUpdateLoop(cfg.Interval, NewMetricsService(), NewHistories(cfg.HistorySize),
		func(m Metrics, h *Histories) {
			uptimeLabel.SetText("  |  Uptime: " + formatUptime(m.Uptime))

			cpuBar.SetValue(m.CPUPercent / 100)
			cpuLabel.SetText(fmt.Sprintf("%.2f%%", m.CPUPercent))
			cpuGraph.Update(h.CPU.GetValues())
			if m.HasTemp {
				cpuTempLabel.SetText(fmt.Sprintf("   🌡 %.1f °C", m.CPUTemp))
			}

			ramBar.SetValue(m.RAMPercent / 100)
			ramLabel.SetText(fmt.Sprintf("%.2f%%  (%.1f / %.1f GB)",
				m.RAMPercent, m.RAMUsed/1024/1024/1024, m.RAMTotal/1024/1024/1024))
			ramGraph.Update(h.RAM.GetValues())

			diskBar.SetValue(m.DiskPercent / 100)
			diskLabel.SetText(fmt.Sprintf("%.2f%%  (%.1f / %.1f GB)",
				m.DiskPercent, m.DiskUsed/1024/1024/1024, m.DiskTotal/1024/1024/1024))
			diskGraph.Update(h.Disk.GetValues())

			netUpLabel.SetText("↑  " + formatSpeed(m.UploadBps))
			netDownLabel.SetText("↓  " + formatSpeed(m.DownloadBps))
			netGraph.Update(h.NetDown.GetValues())
		}, quit)

	w.ShowAndRun()
}

// GraphWidget draws a line graph.
// When autoScale is true the Y-axis adapts to the current max value in the window;
// otherwise values are assumed to be in the range 0–100.
// Values are added to the end of the list, pushing out old values when maxSize is exceeded.
