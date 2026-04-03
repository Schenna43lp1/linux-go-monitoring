//go:build linux

package main

import (
"image/color"
"time"

"fyne.io/fyne/v2"
)

type appConfig struct {
Interval    time.Duration
HistorySize int
WindowSize  fyne.Size
}

var cfg = appConfig{
Interval:    2 * time.Second,
HistorySize: 60,
WindowSize:  fyne.NewSize(900, 660),
}

var (
colorCPU     = color.NRGBA{R: 0, G: 210, B: 100, A: 255}
colorRAM     = color.NRGBA{R: 30, G: 150, B: 255, A: 255}
colorDisk    = color.NRGBA{R: 255, G: 150, B: 0, A: 255}
colorNetDown = color.NRGBA{R: 220, G: 80, B: 255, A: 255}
colorNetUp   = color.NRGBA{R: 255, G: 100, B: 180, A: 255}
)

const (
thresholdWarn = 65.0
thresholdCrit = 85.0
)

var colorGPU = color.NRGBA{R: 255, G: 60, B: 60, A: 255}
var colorVRAM = color.NRGBA{R: 255, G: 200, B: 0, A: 255}
