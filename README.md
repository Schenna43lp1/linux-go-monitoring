# Linux Monitor

A Linux system monitor built with Go, [Fyne](https://fyne.io/) and [gopsutil](https://github.com/shirou/gopsutil).

## Features

- Real-time **CPU**, **RAM**, and **Disk** usage
- Live **line graphs** for each metric (last 2 minutes, updates every 2 s)
- Progress bars with exact percentage and GB values
- Graceful shutdown on window close

## Requirements

- Linux
- Go 1.21+
- Fyne dependencies: `gcc`, `libgl1-mesa-dev`, `xorg-dev`

## Run

```bash
go run main.go
```

## Install

```bash
sudo ./instrall.sh
```

Installs the binary to `/usr/local/bin` and creates a desktop entry.
