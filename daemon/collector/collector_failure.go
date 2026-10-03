package collector

import (
	"errors"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// collectorFailure paces the warning about a feed failing to send to
	// the collector.
	//
	// A collector down fails every send of every feed, every tick: each
	// failure is logged at debug level only, and the feed warns once, then
	// at an interval doubling from warnBackoffMin up to warnBackoffMax while
	// it keeps failing.
	collectorFailure struct {
		// subject names the data of the feed in the messages.
		subject string

		backoff warnBackoff
	}

	// warnBackoff paces a repeated warning: due at once, then after an
	// interval doubling from warnBackoffMin up to warnBackoffMax.
	warnBackoff struct {
		interval time.Duration
		next     time.Time
	}
)

var (
	// warnBackoffMin and warnBackoffMax bound the interval between two
	// warnings of a feed failing to send to the collector.
	warnBackoffMin = 10 * time.Second
	warnBackoffMax = time.Hour

	// errCollectorDataNotSent is a daemon status, change or ping not sent
	// on this tick: throttled, or without a collector client. It says
	// nothing of the collector health.
	errCollectorDataNotSent = errors.New("not sent")

	// errCollectorRefused is the collector refusing data for good: sending
	// it again would not do better.
	errCollectorRefused = errors.New("refused by the collector")
)

// isRefusedStatus tells whether the collector response status code refuses
// the data for good, as a 4xx does, as opposed to a collector failing to
// take it now.
func isRefusedStatus(code int) bool {
	return code >= 400 && code < 500
}

func newCollectorFailure(subject string) collectorFailure {
	return collectorFailure{subject: subject}
}

// update reports the outcome of the latest sends of the feed: err is the
// latest send error, nil when the feed sends fine, and pending the count of
// the items left to send, zero for a feed without items.
//
// A failure warns when due. A success after a warning tells the collector
// accepts the feed again, and starts the pacing over.
func (f *collectorFailure) update(log *plog.Logger, now time.Time, err error, pending int) {
	if err == nil {
		if f.backoff.interval > 0 {
			log.Infof("the collector accepts the %s again", f.subject)
		}
		f.backoff.reset()
		return
	}
	if !f.backoff.due(now) {
		return
	}
	f.backoff.arm(now)
	if pending > 0 {
		log.Warnf("%d %s pending, failing to send to the collector: %v (next warning in %s)", pending, f.subject, err, f.backoff.interval)
	} else {
		log.Warnf("%s failing to send to the collector: %v (next warning in %s)", f.subject, err, f.backoff.interval)
	}
}

// isFailing tells whether the feed warned about a failure not recovered
// yet.
func (f *collectorFailure) isFailing() bool {
	return f.backoff.interval > 0
}

// reset forgets the failure without telling, as when the node stops being
// the collector speaker.
func (f *collectorFailure) reset() {
	f.backoff.reset()
}

func (b *warnBackoff) due(now time.Time) bool {
	return !now.Before(b.next)
}

// arm sets the time the warning is due again, the interval doubled.
func (b *warnBackoff) arm(now time.Time) {
	if b.interval == 0 {
		b.interval = warnBackoffMin
	} else {
		b.interval = min(2*b.interval, warnBackoffMax)
	}
	b.next = now.Add(b.interval)
}

func (b *warnBackoff) reset() {
	*b = warnBackoff{}
}
