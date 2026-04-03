//go:build linux

// card.go – dashCard-Komponente: vereinheitlichte UI-Karte mit Titel, Wert, Progressbar und Graph.
package main

import (
"image/color"

"fyne.io/fyne/v2"
"fyne.io/fyne/v2/container"
"fyne.io/fyne/v2/widget"
)

// dashCard holds the mutable widget references for one metric card.
type dashCard struct {
Container *fyne.Container
Value     *widget.Label
Sub       *widget.Label
Status    *widget.Label
Bar       *widget.ProgressBar
Graph     *GraphWidget
}

// newDashCard builds a dashboard card.
// withBar: show a progress bar. autoScale: graph Y-axis auto-scales.
func newDashCard(title string, graphColor color.Color, withBar bool, autoScale bool) *dashCard {
dc := &dashCard{
Value:  widget.NewLabelWithStyle("–", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
Sub:    widget.NewLabel(""),
Status: widget.NewLabel("🟢"),
}
if autoScale {
dc.Graph = NewAutoScaleGraphWidget(cfg.HistorySize, graphColor)
} else {
dc.Graph = NewGraphWidget(cfg.HistorySize, graphColor)
}
header := container.NewBorder(nil, nil,
widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
dc.Status,
)
items := []fyne.CanvasObject{header, dc.Value, dc.Sub}
if withBar {
dc.Bar = widget.NewProgressBar()
items = append(items, dc.Bar)
}
items = append(items, dc.Graph)
dc.Container = container.NewPadded(container.NewVBox(items...))
return dc
}
