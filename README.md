# Linux Monitor

> A lightweight Linux system monitor with a native GUI — built with Go, [Fyne](https://fyne.io/) and [gopsutil](https://github.com/shirou/gopsutil).

---

## Features

| Metric | Details |
|---|---|
| 🖥 **CPU** | Usage % · temperature (°C) · live graph |
| 🧠 **RAM** | Usage % · used / total GB · live graph |
| 💾 **Disk** | Usage % · used / total GB · live graph |
| 🌐 **Network** | Upload & download speed (B/s · KB/s · MB/s) · live graph |
| ⏱ **Uptime** | Displayed in the title bar (days · hours · minutes) |

- Graphs show the **last 2 minutes** of history, updating every **2 seconds**
- Network graph auto-scales to the current peak speed

---

## Requirements

- Linux (kernel with `/sys/class/hwmon` for temperature)
- Go 1.21+
- Fyne system dependencies:
  ```bash
  sudo apt install gcc libgl1-mesa-dev xorg-dev
  ```

---

## Usage

### Run directly

```bash
go run .
```

### Build & install

```bash
sudo ./instrall.sh
```

Compiles the binary, installs it to `/usr/local/bin`, and creates a `.desktop` entry.

---

## Project structure

```
main.go      — UI: GraphWidget, layout, main()
update.go    — Logic: Config, MetricsService, Histories, RunUpdateLoop
```

Config (update interval, history size, window size) is defined centrally at the top of `update.go`.

---

## Tech stack

- **[Fyne v2](https://fyne.io/)** — cross-platform GUI toolkit
- **[gopsutil v3](https://github.com/shirou/gopsutil)** — system stats (CPU, RAM, disk, network, sensors, uptime)
