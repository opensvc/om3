package collector

import (
	"errors"
	"net/url"
	"time"
)

// isUnreachable says whether err is the failure of a send before any answer
// of the collector, as a name not resolved, a connection refused or timed
// out, rather than an answer refusing it.
func isUnreachable(err error) bool {
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// setUnreachable records the collector unreachable since now, and publishes
// it: the scheduler of every node holds the jobs feeding the collector,
// which would each fail the same way, and pushes the stats, the inventories
// and the checks again once it answers.
func (t *T) setUnreachable(now time.Time) {
	if t.status.UnreachableSince != nil {
		return
	}
	t.log.Warnf("the collector %s is unreachable: the scheduled jobs feeding it are held until it answers", t.status.Url)
	t.status.UnreachableSince = &now
	t.status.UpdatedAt = now
	t.publish()
}

// setReachable records the collector answering, and publishes it.
func (t *T) setReachable() {
	if t.status.UnreachableSince == nil {
		return
	}
	t.log.Infof("the collector %s answers again, unreachable since %s", t.status.Url, t.status.UnreachableSince.Format(time.RFC3339))
	t.status.UnreachableSince = nil
	t.status.UpdatedAt = time.Now()
	t.publish()
}
