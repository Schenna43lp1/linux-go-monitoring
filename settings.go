//go:build linux

// settings.go – Farbkonfiguration: JSON-Datei laden/speichern und Einstellungs-Dialog.
package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// colorFileConfig is the JSON structure for the persisted color configuration.
type colorFileConfig struct {
	CPU     string `json:"cpu"`
	RAM     string `json:"ram"`
	Disk    string `json:"disk"`
	NetDown string `json:"net_down"`
	NetUp   string `json:"net_up"`
	GPU     string `json:"gpu"`
	VRAM    string `json:"vram"`
}

// configFilePath returns the path to the JSON config file
// (~/.config/linux-monitor/config.json).
func configFilePath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "linux-monitor", "config.json")
}

// loadColorConfig reads the JSON config file and applies stored colors to the
// global color variables. Missing or invalid entries are silently ignored so
// defaults from config.go remain in effect.
func loadColorConfig() {
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return
	}
	var s colorFileConfig
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	if c, ok := hexToNRGBA(s.CPU); ok {
		colorCPU = c
	}
	if c, ok := hexToNRGBA(s.RAM); ok {
		colorRAM = c
	}
	if c, ok := hexToNRGBA(s.Disk); ok {
		colorDisk = c
	}
	if c, ok := hexToNRGBA(s.NetDown); ok {
		colorNetDown = c
	}
	if c, ok := hexToNRGBA(s.NetUp); ok {
		colorNetUp = c
	}
	if c, ok := hexToNRGBA(s.GPU); ok {
		colorGPU = c
	}
	if c, ok := hexToNRGBA(s.VRAM); ok {
		colorVRAM = c
	}
}

// saveColorConfig writes the current global color variables to the JSON config
// file, creating parent directories as needed.
func saveColorConfig() error {
	s := colorFileConfig{
		CPU:     nrgbaToHex(colorCPU),
		RAM:     nrgbaToHex(colorRAM),
		Disk:    nrgbaToHex(colorDisk),
		NetDown: nrgbaToHex(colorNetDown),
		NetUp:   nrgbaToHex(colorNetUp),
		GPU:     nrgbaToHex(colorGPU),
		VRAM:    nrgbaToHex(colorVRAM),
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := configFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// hexToNRGBA parses a CSS hex color string ("#rrggbb") into color.NRGBA.
func hexToNRGBA(s string) (color.NRGBA, bool) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, false
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{R: r, G: g, B: b, A: 255}, true
}

// nrgbaToHex converts a color.NRGBA to a CSS hex string ("#rrggbb").
func nrgbaToHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// toNRGBA converts any color.Color to color.NRGBA.
func toNRGBA(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

// ── colorSwatch ──────────────────────────────────────────────────────────────

// colorSwatch is a small colored rectangle widget used as a preview in the
// settings dialog.
type colorSwatch struct {
	widget.BaseWidget
	rect *canvas.Rectangle
}

func newColorSwatch(c color.Color) *colorSwatch {
	s := &colorSwatch{rect: canvas.NewRectangle(c)}
	s.ExtendBaseWidget(s)
	return s
}

// SetColor updates the swatch's displayed color.
func (s *colorSwatch) SetColor(c color.Color) {
	s.rect.FillColor = c
	s.rect.Refresh()
}

func (s *colorSwatch) MinSize() fyne.Size        { return fyne.NewSize(44, 24) }
func (s *colorSwatch) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.rect)
}

// ── colorEntry ───────────────────────────────────────────────────────────────

// colorEntry binds a UI label to the global color variable it controls and
// the graph widgets that should be repainted when the color changes.
type colorEntry struct {
	label    string
	colorPtr *color.NRGBA
	graphs   []*GraphWidget
}

// apply writes the new color to the global variable and repaints all bound graphs.
func (e *colorEntry) apply(c color.Color) {
	nrgba := toNRGBA(c)
	*e.colorPtr = nrgba
	for _, g := range e.graphs {
		g.SetColor(nrgba)
	}
}

// ── settings dialog ──────────────────────────────────────────────────────────

// showSettingsDialog opens a modal dialog that lets the user pick a new color
// for each metric. Changes are applied immediately and persisted to disk.
func showSettingsDialog(win fyne.Window, entries []colorEntry) {
	rows := make([]fyne.CanvasObject, 0, len(entries))

	for i := range entries {
		e := &entries[i]

		swatch := newColorSwatch(*e.colorPtr)
		hexLabel := widget.NewLabel(nrgbaToHex(*e.colorPtr))

		editBtn := widget.NewButton("✎ Change", func() {
			picker := dialog.NewColorPicker(
				"Color: "+e.label, "",
				func(c color.Color) {
					e.apply(c)
					nrgba := *e.colorPtr
					swatch.SetColor(nrgba)
					hexLabel.SetText(nrgbaToHex(nrgba))
					_ = saveColorConfig()
				}, win)
			picker.Advanced = true
			picker.Show()
		})

		row := container.NewBorder(nil, nil,
			widget.NewLabel(e.label),
			editBtn,
			container.NewHBox(swatch, hexLabel),
		)
		rows = append(rows, row, widget.NewSeparator())
	}

	content := container.NewVBox(rows...)
	dlg := dialog.NewCustom("⚙  Color Settings", "Close", content, win)
	dlg.Resize(fyne.NewSize(420, 0))
	dlg.Show()
}
