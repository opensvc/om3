package daemonapi

import (
	"net"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// authRefusals paces the log of the requests refused authentication,
	// per client address: a client sending no credentials, as a metrics
	// scraper left without them, is refused at each of its polls, every
	// few seconds, and was logged at the error level each time. The
	// refusal is the client's, and worth a warning; the repeats are
	// counted, and summed up once per interval. The reason is not of the
	// key: it holds what the client sent, as a username, and a client
	// changing it at each request would escape the pacing.
	authRefusals struct {
		mu       sync.Mutex
		interval time.Duration
		m        map[string]*authRefusal
	}

	authRefusal struct {
		loggedAt   time.Time
		lastSeenAt time.Time
		count      int
	}
)

// authRefusalsMax bounds the clients remembered: beyond, the ones not seen
// for an interval are forgotten.
const authRefusalsMax = 1024

var refusedAuth = &authRefusals{interval: 10 * time.Minute, m: make(map[string]*authRefusal)}

// log logs the refusal of the request of path from the remote address.
func (t *authRefusals) log(log *plog.Logger, now time.Time, remoteAddr, path string, err error) {
	host := remoteAddr
	if h, _, splitErr := net.SplitHostPort(remoteAddr); splitErr == nil {
		host = h
	}
	k := host
	t.mu.Lock()
	defer t.mu.Unlock()
	r, ok := t.m[k]
	if !ok && len(t.m) >= authRefusalsMax {
		t.evict(now)
	}
	switch {
	case !ok:
		t.m[k] = &authRefusal{loggedAt: now, lastSeenAt: now}
		log.Warnf("authenticating request %s from %s: %s", path, remoteAddr, err)
		return
	case now.Sub(r.loggedAt) >= t.interval:
		log.Warnf("authenticating request %s from %s: %s (%d more requests from %s refused since %s)",
			path, remoteAddr, err, r.count, host, r.loggedAt.Format(time.RFC3339))
		r.loggedAt = now
		r.count = 0
	default:
		r.count++
		log.Debugf("authenticating request %s from %s: %s", path, remoteAddr, err)
	}
	r.lastSeenAt = now
}

// evict forgets the clients not seen for an interval, else the one seen the
// longest ago: a client changing its refused username at each request does
// not grow the map past authRefusalsMax. The caller holds the lock.
func (t *authRefusals) evict(now time.Time) {
	var oldestKey string
	var oldestAt time.Time
	for key, r := range t.m {
		if now.Sub(r.lastSeenAt) >= t.interval {
			delete(t.m, key)
			continue
		}
		if oldestKey == "" || r.lastSeenAt.Before(oldestAt) {
			oldestKey, oldestAt = key, r.lastSeenAt
		}
	}
	if len(t.m) >= authRefusalsMax && oldestKey != "" {
		delete(t.m, oldestKey)
	}
}
