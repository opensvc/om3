package daemondata

import (
	"context"

	"opensvc.com/opensvc/util/callcount"
)

type opStats struct {
	stats chan<- callcount.Stats
}

func (o opStats) call(ctx context.Context, d *data) {
	d.counterCmd <- idStats
	select {
	case <-ctx.Done():
	case o.stats <- callcount.GetStats(d.counterCmd):
	}
}

func (t T) Stats() callcount.Stats {
	stats := make(chan callcount.Stats)
	t.cmdC <- opStats{stats: stats}
	return <-stats
}
