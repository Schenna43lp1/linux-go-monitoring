//go:build linux

package main

import "fmt"

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
