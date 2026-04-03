//go:build linux

// model.go – Datenstrukturen für alle Metriken (CPU, RAM, Disk, Network, GPU, System).
package main

// Metrics is a snapshot of all system metrics.
type Metrics struct {
	CPUPercent  float64
	CPUTemp     float64
	HasTemp     bool
	RAMPercent  float64
	RAMUsed     float64
	RAMTotal    float64
	DiskPercent float64
	DiskUsed    float64
	DiskTotal   float64
	UploadBps   float64
	DownloadBps float64
	Uptime      uint64
	GPU         GPUInfo
	Processes   []ProcessInfo
	Err         error
}

// ProcessInfo holds information about a single running process.
type ProcessInfo struct {
	PID    int32
	Name   string
	CPUPct float64
	RAMPct float64
	RAMMB  float64
}

// GPUInfo holds GPU utilization and VRAM data for one GPU.
type GPUInfo struct {
Name        string
UtilPercent float64
VRAMUsed    float64 // bytes
VRAMTotal   float64 // bytes
VRAMPercent float64
Temp        float64
HasGPU      bool
}

// SystemInfo holds static system information.
type SystemInfo struct {
Hostname string
OS       string
Kernel   string
}
