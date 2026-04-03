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

// ── Config ───────────────────────────────────────────────────────────────────

type appConfig struct {
	Interval    time.Duration
	HistorySize int
	WindowSize  fyne.Size
}

var cfg = appConfig{
	Interval:    2 * time.Second,
	HistorySize: 60,
	WindowSize:  fyne.NewSize(820, 480),
}

// ── Metrics (Service Layer) ───────────────────────────────────────────────────

// Metrics is a single snapshot of all system metrics.
type Metrics struct {
	CPUPercent  float64
	CPUTemp     float64
	HasTemp     bool
	RAMPercent  float64
	RAMUsed     float64 // bytes
	RAMTotal    float64 // bytes
	DiskPercent float64
	DiskUsed    float64 // bytes
	DiskTotal   float64 // bytes
	UploadBps   float64
	DownloadBps float64
	Uptime      uint64 // seconds
}

// MetricsService collects system metrics. It keeps track of network counters
// between calls to compute upload/download speeds.
type MetricsService struct {
	prevSent    uint64
	prevRecv    uint64
	prevNetTime time.Time
}

func NewMetricsService() *MetricsService {
	svc := &MetricsService{}
	if ctrs, err := psnet.IOCounters(false); err == nil && len(ctrs) > 0 {
		svc.prevSent = ctrs[0].BytesSent
		svc.prevRecv = ctrs[0].BytesRecv
		svc.prevNetTime = time.Now()
	}
	return svc
}

// Collect samples all metrics and returns a Metrics snapshot.
func (s *MetricsService) Collect() Metrics {
	var m Metrics

	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		m.CPUPercent = pct[0]
	}
	m.CPUTemp, m.HasTemp = getCPUTemp()

	if vmem, err := mem.VirtualMemory(); err == nil {
		m.RAMPercent = vmem.UsedPercent
		m.RAMUsed = float64(vmem.Used)
		m.RAMTotal = float64(vmem.Total)
	}

	if d, err := disk.Usage("/"); err == nil {
		m.DiskPercent = d.UsedPercent
		m.DiskUsed = float64(d.Used)
		m.DiskTotal = float64(d.Total)
	}

	m.Uptime, _ = host.Uptime()

	if ctrs, err := psnet.IOCounters(false); err == nil && len(ctrs) > 0 {
		now := time.Now()
		if dt := now.Sub(s.prevNetTime).Seconds(); dt > 0 {
			m.UploadBps = float64(ctrs[0].BytesSent-s.prevSent) / dt
			m.DownloadBps = float64(ctrs[0].BytesRecv-s.prevRecv) / dt
		}
		s.prevSent = ctrs[0].BytesSent
		s.prevRecv = ctrs[0].BytesRecv
		s.prevNetTime = now
	}

	return m
}

// ── Histories ────────────────────────────────────────────────────────────────

// Histories bundles the rolling-window histories for every metric.
type Histories struct {
	CPU     *MetricHistory
	RAM     *MetricHistory
	Disk    *MetricHistory
	NetDown *MetricHistory
}

func NewHistories(size int) *Histories {
	return &Histories{
		CPU:     NewMetricHistory(size),
		RAM:     NewMetricHistory(size),
		Disk:    NewMetricHistory(size),
		NetDown: NewMetricHistory(size),
	}
}

func (h *Histories) Update(m Metrics) {
	h.CPU.Add(m.CPUPercent)
	h.RAM.Add(m.RAMPercent)
	h.Disk.Add(m.DiskPercent)
	h.NetDown.Add(m.DownloadBps)
}

// ── Update Loop ───────────────────────────────────────────────────────────────

// RunUpdateLoop runs in a goroutine: collects metrics every interval,
// updates histories, then calls onUpdate on the Fyne main goroutine.
func RunUpdateLoop(
	interval time.Duration,
	svc *MetricsService,
	h *Histories,
	onUpdate func(Metrics, *Histories),
	quit <-chan struct{},
) {
	cpu.Percent(0, false) // warmup: initialises the baseline for the first measurement
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			m := svc.Collect()
			h.Update(m)
			fyne.Do(func() { onUpdate(m, h) })
		}
	}
}

// ── MetricHistory ─────────────────────────────────────────────────────────────

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

	// Update loop
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
