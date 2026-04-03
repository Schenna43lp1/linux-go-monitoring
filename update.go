//go:build linux

package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
)

// ── Config ────────────────────────────────────────────────────────────────────

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

// ── Metrics ───────────────────────────────────────────────────────────────────

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

// ── MetricsService ────────────────────────────────────────────────────────────

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

// ── Histories ─────────────────────────────────────────────────────────────────

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

// ── Helpers ───────────────────────────────────────────────────────────────────

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
