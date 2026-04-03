//go:build linux

package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ── Theme ─────────────────────────────────────────────────────────────────────

var (
	colorCPU     = color.NRGBA{R: 0, G: 210, B: 100, A: 255}
	colorRAM     = color.NRGBA{R: 30, G: 150, B: 255, A: 255}
	colorDisk    = color.NRGBA{R: 255, G: 150, B: 0, A: 255}
	colorNetDown = color.NRGBA{R: 220, G: 80, B: 255, A: 255}
	colorNetUp   = color.NRGBA{R: 255, G: 100, B: 180, A: 255}

	thresholdWarn = 65.0
	thresholdCrit = 85.0
)

func statusDot(pct float64) string {
	switch {
	case pct >= thresholdCrit:
		return "🔴"
	case pct >= thresholdWarn:
		return "🟡"
	default:
		return "🟢"
	}
}

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

func (r *graphRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 110) }

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

// ── dashCard ──────────────────────────────────────────────────────────────────

// dashCard holds the mutable widget references for one metric card.
type dashCard struct {
	Container *fyne.Container
	Value     *widget.Label
	Sub       *widget.Label
	Status    *widget.Label
	Bar       *widget.ProgressBar
	Graph     *GraphWidget
}

// newDashCard builds a dashboard card.
// withBar: show a progress bar. autoScale: graph Y-axis auto-scales.
func newDashCard(title string, graphColor color.Color, withBar bool, autoScale bool) *dashCard {
	dc := &dashCard{
		Value:  widget.NewLabelWithStyle("–", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		Sub:    widget.NewLabel(""),
		Status: widget.NewLabel("🟢"),
	}
	if autoScale {
		dc.Graph = NewAutoScaleGraphWidget(cfg.HistorySize, graphColor)
	} else {
		dc.Graph = NewGraphWidget(cfg.HistorySize, graphColor)
	}
	header := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		dc.Status,
	)
	items := []fyne.CanvasObject{header, dc.Value, dc.Sub}
	if withBar {
		dc.Bar = widget.NewProgressBar()
		items = append(items, dc.Bar)
	}
	items = append(items, dc.Graph)
	dc.Container = container.NewPadded(container.NewVBox(items...))
	return dc
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	a := app.New()
	w := a.NewWindow("Linux Monitor")
	w.Resize(cfg.WindowSize)

	// ── Overview Tab ──────────────────────────────────────────────────────────
	cpuCard  := newDashCard("🖥  CPU",     colorCPU,     true,  false)
	ramCard  := newDashCard("🧠  RAM",     colorRAM,     true,  false)
	diskCard := newDashCard("💾  DISK",    colorDisk,    true,  false)
	netCard  := newDashCard("🌐  NETWORK", colorNetDown, false, true)

	overviewTab := container.NewTabItem("Overview",
		container.NewScroll(container.NewGridWithColumns(2,
			cpuCard.Container,
			ramCard.Container,
			diskCard.Container,
			netCard.Container,
		)),
	)

	// ── Network Tab ───────────────────────────────────────────────────────────
	netDownCard := newDashCard("↓  Download", colorNetDown, false, true)
	netUpCard   := newDashCard("↑  Upload",   colorNetUp,   false, true)

	networkTab := container.NewTabItem("Network",
		container.NewScroll(container.NewVBox(
			netDownCard.Container,
			netUpCard.Container,
		)),
	)

	// ── System Tab ────────────────────────────────────────────────────────────
	sysInfo        := getSystemInfo()
	uptimeValLabel := widget.NewLabel("–")

	systemTab := container.NewTabItem("System",
		container.NewScroll(container.NewPadded(container.NewVBox(
			widget.NewLabelWithStyle("System Information", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			container.NewGridWithColumns(2,
				widget.NewLabel("Hostname"), widget.NewLabel(sysInfo.Hostname),
				widget.NewLabel("OS"),       widget.NewLabel(sysInfo.OS),
				widget.NewLabel("Kernel"),   widget.NewLabel(sysInfo.Kernel),
				widget.NewLabel("Uptime"),   uptimeValLabel,
			),
		))),
	)

	tabs := container.NewAppTabs(overviewTab, networkTab, systemTab)

	// ── Header ────────────────────────────────────────────────────────────────
	uptimeLabel := widget.NewLabel("")
	header := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("🖥  Linux Monitor", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		uptimeLabel,
	)

	// ── Alert Banner ──────────────────────────────────────────────────────────
	alertLabel := widget.NewLabel("")

	// ── Footer ────────────────────────────────────────────────────────────────
	footerLabel := widget.NewLabelWithStyle("", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true})

	w.SetContent(container.NewBorder(
		container.NewVBox(header, widget.NewSeparator(), alertLabel, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), footerLabel),
		nil, nil,
		tabs,
	))

	// ── Update Loop ───────────────────────────────────────────────────────────
	quit := make(chan struct{})
	w.SetOnClosed(func() { close(quit) })

	go RunUpdateLoop(cfg.Interval, NewMetricsService(), NewHistories(cfg.HistorySize),
		func(m Metrics, h *Histories) {
			now := time.Now()
			uptimeLabel.SetText("  |  Uptime: " + formatUptime(m.Uptime))
			uptimeValLabel.SetText(formatUptime(m.Uptime))
			footerLabel.SetText("Last updated: " + now.Format("15:04:05"))

			// Alert banner
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

			// CPU card
			cpuCard.Status.SetText(statusDot(m.CPUPercent))
			cpuCard.Value.SetText(fmt.Sprintf("%.1f%%", m.CPUPercent))
			if m.HasTemp {
				cpuCard.Sub.SetText(fmt.Sprintf("🌡  %.1f °C", m.CPUTemp))
			}
			cpuCard.Bar.SetValue(m.CPUPercent / 100)
			cpuCard.Graph.Update(h.CPU.GetValues())

			// RAM card
			ramCard.Status.SetText(statusDot(m.RAMPercent))
			ramCard.Value.SetText(fmt.Sprintf("%.1f%%", m.RAMPercent))
			ramCard.Sub.SetText(fmt.Sprintf("%.1f / %.1f GB",
				m.RAMUsed/1024/1024/1024, m.RAMTotal/1024/1024/1024))
			ramCard.Bar.SetValue(m.RAMPercent / 100)
			ramCard.Graph.Update(h.RAM.GetValues())

			// Disk card
			diskCard.Status.SetText(statusDot(m.DiskPercent))
			diskCard.Value.SetText(fmt.Sprintf("%.1f%%", m.DiskPercent))
			diskCard.Sub.SetText(fmt.Sprintf("%.1f / %.1f GB",
				m.DiskUsed/1024/1024/1024, m.DiskTotal/1024/1024/1024))
			diskCard.Bar.SetValue(m.DiskPercent / 100)
			diskCard.Graph.Update(h.Disk.GetValues())

			// Network overview card
			netCard.Value.SetText(fmt.Sprintf("↓  %s", formatSpeed(m.DownloadBps)))
			netCard.Sub.SetText(fmt.Sprintf("↑  %s", formatSpeed(m.UploadBps)))
			netCard.Graph.Update(h.NetDown.GetValues())

			// Network detail cards
			netDownCard.Value.SetText(formatSpeed(m.DownloadBps))
			netDownCard.Sub.SetText("Download speed (auto-scale)")
			netDownCard.Graph.Update(h.NetDown.GetValues())

			netUpCard.Value.SetText(formatSpeed(m.UploadBps))
			netUpCard.Sub.SetText("Upload speed (auto-scale)")
			netUpCard.Graph.Update(h.NetUp.GetValues())
		}, quit)

	w.ShowAndRun()
}
