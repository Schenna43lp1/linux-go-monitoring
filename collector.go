//go:build linux

// collector.go – sammelt Systemmetriken via gopsutil, nvidia-smi und sysfs (Collector-Interface + Implementierung).
package main

import (
"fmt"
	"os"
	"os/exec"
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
m.Disks = collectDisks()

m.Uptime, _ = host.Uptime()
	m.GPU = gpuInfo()
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

func collectDisks() []DiskPartition {
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil
	}
	// filesystems to skip (virtual/pseudo)
	skipFS := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "devfs": true, "overlay": true,
		"aufs": true, "squashfs": true, "proc": true, "sysfs": true,
		"cgroup": true, "cgroup2": true, "pstore": true, "securityfs": true,
		"debugfs": true, "hugetlbfs": true, "mqueue": true, "fusectl": true,
		"efivarfs": true, "bpf": true, "tracefs": true,
	}
	var result []DiskPartition
	for _, p := range parts {
		if skipFS[p.Fstype] {
			continue
		}
		mp := p.Mountpoint
		if strings.HasPrefix(mp, "/sys") || strings.HasPrefix(mp, "/proc") ||
			strings.HasPrefix(mp, "/dev") || strings.HasPrefix(mp, "/run") {
			continue
		}
		usage, err := disk.Usage(mp)
		if err != nil || usage.Total == 0 {
			continue
		}
		result = append(result, DiskPartition{
			Mount:   mp,
			Percent: usage.UsedPercent,
			Used:    float64(usage.Used),
			Total:   float64(usage.Total),
		})
	}
	return result
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

// gpuInfo tries NVIDIA first (nvidia-smi), then AMD sysfs.
func gpuInfo() GPUInfo {
if g, ok := nvidiaSMI(); ok {
return g
}
if g, ok := amdSysfs(); ok {
return g
}
return GPUInfo{}
}

func nvidiaSMI() (GPUInfo, bool) {
out, err := exec.Command("nvidia-smi",
"--query-gpu=name,utilization.gpu,memory.used,memory.total,temperature.gpu",
"--format=csv,noheader,nounits").Output()
if err != nil {
return GPUInfo{}, false
}
parts := strings.SplitN(strings.TrimSpace(string(out)), ",", 5)
if len(parts) < 5 {
return GPUInfo{}, false
}
parse := func(s string) float64 {
var f float64
fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
return f
}
vramUsed := parse(parts[2]) * 1024 * 1024
vramTotal := parse(parts[3]) * 1024 * 1024
vramPct := 0.0
if vramTotal > 0 {
vramPct = vramUsed / vramTotal * 100
}
return GPUInfo{
Name:        strings.TrimSpace(parts[0]),
UtilPercent: parse(parts[1]),
VRAMUsed:    vramUsed,
VRAMTotal:   vramTotal,
VRAMPercent: vramPct,
Temp:        parse(parts[4]),
HasGPU:      true,
}, true
}

func amdSysfs() (GPUInfo, bool) {
utilBytes, err := os.ReadFile("/sys/class/drm/card0/device/gpu_busy_percent")
if err != nil {
return GPUInfo{}, false
}
var util float64
fmt.Sscanf(strings.TrimSpace(string(utilBytes)), "%f", &util)

var vramUsed, vramTotal float64
if b, err := os.ReadFile("/sys/class/drm/card0/device/mem_info_vram_used"); err == nil {
fmt.Sscanf(strings.TrimSpace(string(b)), "%f", &vramUsed)
}
if b, err := os.ReadFile("/sys/class/drm/card0/device/mem_info_vram_total"); err == nil {
fmt.Sscanf(strings.TrimSpace(string(b)), "%f", &vramTotal)
}
vramPct := 0.0
if vramTotal > 0 {
vramPct = vramUsed / vramTotal * 100
}

var temp float64
if b, err := os.ReadFile("/sys/class/drm/card0/device/hwmon/hwmon0/temp1_input"); err == nil {
fmt.Sscanf(strings.TrimSpace(string(b)), "%f", &temp)
temp /= 1000
}

return GPUInfo{
Name:        "AMD GPU",
UtilPercent: util,
VRAMUsed:    vramUsed,
VRAMTotal:   vramTotal,
VRAMPercent: vramPct,
Temp:        temp,
HasGPU:      true,
}, true
}
