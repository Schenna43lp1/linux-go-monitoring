//go:build linux

package main

import (
"fmt"
"strings"
"time"

"github.com/shirou/gopsutil/v3/cpu"
"github.com/shirou/gopsutil/v3/disk"
"github.com/shirou/gopsutil/v3/host"
"github.com/shirou/gopsutil/v3/mem"
psnet "github.com/shirou/gopsutil/v3/net"
)

// Collector is the interface for fetching system metrics.
type Collector interface {
Collect() Metrics
SystemInfo() SystemInfo
}

// systemCollector implements Collector using gopsutil.
type systemCollector struct {
prevSent    uint64
prevRecv    uint64
prevNetTime time.Time
}

func newCollector() Collector {
sc := &systemCollector{}
if ctrs, err := psnet.IOCounters(false); err == nil && len(ctrs) > 0 {
sc.prevSent = ctrs[0].BytesSent
sc.prevRecv = ctrs[0].BytesRecv
sc.prevNetTime = time.Now()
}
return sc
}

func (s *systemCollector) Collect() Metrics {
var m Metrics

if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
m.CPUPercent = pct[0]
}
m.CPUTemp, m.HasTemp = cpuTemp()

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
m.UploadBps, m.DownloadBps = netRate(s)

return m
}

func (s *systemCollector) SystemInfo() SystemInfo {
info, err := host.Info()
if err != nil {
return SystemInfo{Hostname: "–", OS: "–", Kernel: "–"}
}
return SystemInfo{
Hostname: info.Hostname,
OS:       fmt.Sprintf("%s %s", info.Platform, info.PlatformVersion),
Kernel:   info.KernelVersion,
}
}

func cpuTemp() (float64, bool) {
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
maxTemp := temps[0].Temperature
for _, t := range temps[1:] {
if t.Temperature > maxTemp {
maxTemp = t.Temperature
}
}
return maxTemp, true
}

func netRate(s *systemCollector) (upload, download float64) {
ctrs, err := psnet.IOCounters(false)
if err != nil || len(ctrs) == 0 {
return
}
now := time.Now()
if dt := now.Sub(s.prevNetTime).Seconds(); dt > 0 {
upload = float64(ctrs[0].BytesSent-s.prevSent) / dt
download = float64(ctrs[0].BytesRecv-s.prevRecv) / dt
}
s.prevSent = ctrs[0].BytesSent
s.prevRecv = ctrs[0].BytesRecv
s.prevNetTime = now
return
}
