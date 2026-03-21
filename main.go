package main

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

func formatUptime(seconds uint64) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func colorize(percent float64) string {
	if percent < 50 {
		return color.GreenString("%.2f %%", percent)
	} else if percent < 80 {
		return color.YellowString("%.2f %%", percent)
	}
	return color.RedString("%.2f %%", percent)
}

func main() {
	for {
		fmt.Print("\033[H\033[2J") // Clear screen

		cpuPercent, _ := cpu.Percent(0, false)
		vmem, _ := mem.VirtualMemory()
		diskStat, _ := disk.Usage("/")
		loadStat, _ := load.Avg()
		uptime, _ := host.Uptime()

		fmt.Println("╔══════════════════════════════╗")
		fmt.Println("║   🖥️  LINUX MONITOR v1       ║")
		fmt.Println("╠══════════════════════════════╣")

		if len(cpuPercent) > 0 {
			fmt.Printf(" CPU     │ %s\n", colorize(cpuPercent[0]))
		}

		fmt.Printf(" RAM     │ %s  (%.1f / %.1f GB)\n",
			colorize(vmem.UsedPercent),
			float64(vmem.Used)/1024/1024/1024,
			float64(vmem.Total)/1024/1024/1024,
		)

		fmt.Printf(" DISK    │ %s  (%.0f / %.0f GB)\n",
			colorize(diskStat.UsedPercent),
			float64(diskStat.Used)/1024/1024/1024,
			float64(diskStat.Total)/1024/1024/1024,
		)

		fmt.Printf(" LOAD    │ %.2f | %.2f | %.2f\n",
			loadStat.Load1,
			loadStat.Load5,
			loadStat.Load15,
		)

		fmt.Printf(" UPTIME  │ %s\n", formatUptime(uptime))

		fmt.Println("╚══════════════════════════════╝")

		time.Sleep(2 * time.Second)
	}
}
