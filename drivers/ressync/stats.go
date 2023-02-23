package ressync

import "time"

type (
	Stats struct {
		SentBytes     uint64
		ReceivedBytes uint64
		Begin         time.Time
		End           time.Time
	}
)

func NewStats() *Stats {
	stats := Stats{
		Begin: time.Now(),
	}
	return &stats
}

func (t *Stats) Close() {
	t.End = time.Now()
}

func (t Stats) Duration() (duration time.Duration) {
	if t.End.IsZero() {
		return
	}
	duration = t.End.Sub(t.Begin)
	return
}

func (t Stats) SpeedBPS() (speed float64) {
	duration := t.Duration()
	if duration == 0 {
		return
	}
	speed = float64(t.SentBytes+t.ReceivedBytes) / duration.Seconds()
	return
}
