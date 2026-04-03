//go:build linux

package main

import (
"context"
"time"

"fyne.io/fyne/v2"
)

// RunLoop starts the metric collection loop. It runs until ctx is cancelled.
// onUpdate is called on the Fyne main goroutine after each collection cycle.
func RunLoop(ctx context.Context, c Collector, h *Histories, onUpdate func(Metrics, *Histories)) {
c.Collect() // warmup
ticker := time.NewTicker(cfg.Interval)
defer ticker.Stop()
for {
select {
case <-ctx.Done():
return
case <-ticker.C:
m := c.Collect()
h.Record(m)
fyne.Do(func() { onUpdate(m, h) })
}
}
}
