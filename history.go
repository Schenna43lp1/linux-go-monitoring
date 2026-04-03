//go:build linux

package main

import "sync"

// MetricHistory is a thread-safe rolling window of float64 values.
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

// Values returns a copy of the current history values.
func (h *MetricHistory) Values() []float64 {
h.mu.Lock()
defer h.mu.Unlock()
out := make([]float64, len(h.values))
copy(out, h.values)
return out
}

// Histories bundles rolling windows for all tracked metrics.
type Histories struct {
CPU     *MetricHistory
RAM     *MetricHistory
Disk    *MetricHistory
NetDown *MetricHistory
NetUp   *MetricHistory
}

func NewHistories(size int) *Histories {
return &Histories{
CPU:     NewMetricHistory(size),
RAM:     NewMetricHistory(size),
Disk:    NewMetricHistory(size),
NetDown: NewMetricHistory(size),
NetUp:   NewMetricHistory(size),
}
}

// Record adds a metrics snapshot to all history windows.
func (h *Histories) Record(m Metrics) {
h.CPU.Add(m.CPUPercent)
h.RAM.Add(m.RAMPercent)
h.Disk.Add(m.DiskPercent)
h.NetDown.Add(m.DownloadBps)
h.NetUp.Add(m.UploadBps)
}
