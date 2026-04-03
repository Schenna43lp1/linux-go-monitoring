//go:build linux

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
Err         error
}

// SystemInfo holds static system information.
type SystemInfo struct {
Hostname string
OS       string
Kernel   string
}
