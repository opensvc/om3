package ressync

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/opensvc/om3/util/progress"
	"github.com/opensvc/om3/util/sizeconv"
)

type (
	Stats struct {
		Endpoint      string
		SentBytes     uint64
		ReceivedBytes uint64
		Begin         time.Time
		End           time.Time
	}
)

func (t *T) CopyWithStats(ctx context.Context, dst io.Writer, src io.Reader, stats *Stats) (uint64, error) {
	buf := make([]byte, 8192)
	q := make(chan any)

	progressRoutine := func(q chan any) {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				t.ProgressStats(ctx, stats)
			case <-q:
				return
			}
		}
	}

	poisonProgressRoutine := func() { q <- nil }

	go progressRoutine(q)
	defer poisonProgressRoutine()
	defer t.ProgressStats(ctx, stats)
	defer t.Log().
		Attr("speed_bps", stats.SpeedBPS()).
		Attr("duration", stats.Duration()).
		Attr("sent_b", stats.SentBytes).
		Attr("received_b", stats.ReceivedBytes).
		Infof("sync stat")

	for {
		n, err := src.Read(buf)
		stats.SentBytes += uint64(n)
		if err != nil && err != io.EOF {
			return stats.SentBytes, err
		}
		if n == 0 {
			break
		}
		if _, err := dst.Write(buf[:n]); err != nil {
			return stats.SentBytes, err
		}
	}
	return stats.SentBytes, nil
}

func (t *T) ProgressNode(ctx context.Context, nodename string, cols ...any) {
	if view := progress.ViewFromContext(ctx); view != nil {
		key := append(t.ProgressKey(), nodename)
		view.Info(key, cols)
	}
}

func (t *T) ProgressStats(ctx context.Context, stats *Stats) {
	rx := fmt.Sprintf("rx:%s", sizeconv.BSizeCompact(float64(stats.ReceivedBytes)))
	tx := fmt.Sprintf("tx:%s", sizeconv.BSizeCompact(float64(stats.SentBytes)))
	t.ProgressNode(ctx, stats.Endpoint, "▶", rx, tx)
}

func NewStats(endpoint string) *Stats {
	stats := Stats{
		Endpoint: endpoint,
		Begin:    time.Now(),
	}
	return &stats
}

func (t *Stats) Close() {
	t.End = time.Now()
}

func (t *Stats) Duration() (duration time.Duration) {
	if t.End.IsZero() {
		return
	}
	duration = t.End.Sub(t.Begin)
	return
}

func (t *Stats) SpeedBPS() (speed float64) {
	duration := t.Duration()
	if duration == 0 {
		return
	}
	speed = float64(t.SentBytes+t.ReceivedBytes) / duration.Seconds()
	return
}
