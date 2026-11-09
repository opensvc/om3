package collector

import (
	"time"
)

const (
	// credsRefusedAfter is how long the collector answers 401 to every
	// request of the node, and nothing else, before its credentials are
	// taken for refused: a collector restarting or an authentication
	// backend failing a moment does not move the speech.
	credsRefusedAfter = 1 * time.Minute

	// credsRefusedMin and credsRefusedMax bound how long credentials taken
	// for refused leave the speech to the other nodes, before the node
	// offers to speak again: the first time for credsRefusedMin, then for
	// twice as long each time the collector refuses them again.
	credsRefusedMin = 5 * time.Minute
	credsRefusedMax = 30 * time.Minute
)

// unauthorizedSince returns when the current run of 401 answers to the node
// began, zero when the last answer was another.
func (t *T) unauthorizedSince() time.Time {
	if r, ok := t.client.(*instrumentedRequester); ok {
		if n := r.unauthorizedSince.Load(); n != 0 {
			return time.Unix(0, n)
		}
	}
	return time.Time{}
}

// acceptedAt returns when the collector last answered the node otherwise
// than 401, zero when it never did.
func (t *T) acceptedAt() time.Time {
	if r, ok := t.client.(*instrumentedRequester); ok {
		if n := r.acceptedAt.Load(); n != 0 {
			return time.Unix(0, n)
		}
	}
	return time.Time{}
}

// credsRefused says whether the collector refuses the credentials of the
// node, its node.uuid: it can not speak.
func (t *T) credsRefused() bool {
	return !t.credsRefusedUntil.IsZero()
}

// checkCredentials leaves the speech to the other nodes while the collector
// refuses the credentials of the node, as a node.uuid the collector does not
// know: each node has its own, and the next one able to speak takes over.
//
// The credentials are taken for refused after credsRefusedAfter of 401
// answers. They are offered again once the backoff ends, or at once with a
// new node.uuid, which applyConfig sees. With every node refused, the leader
// speaks, its state saying the warning.
//
// It is called by the loop, on the refresh ticker.
func (t *T) checkCredentials(now time.Time) {
	if t.credsRefused() {
		if now.Before(t.credsRefusedUntil) {
			// The credentials stay refused until credsRefusedUntil.
			return
		}
		// Reset the refusal time and record the time of the last offer.
		t.credsRefusedUntil = time.Time{}
		t.credsOfferedAt = now
		if r, ok := t.client.(*instrumentedRequester); ok {
			// The 401 run before the backoff is no evidence anymore:
			// the node is judged on the answers to come.
			r.unauthorizedSince.Store(0)
		}
		t.log.Infof("offer to speak to the collector again, %s after its refusal of node.uuid", t.credsRefusedBackoff)
		t.updateSpeaker()
		t.publishOnChange(t.getState())
		return
	}
	// !t.credsRefused() means t.credsRefusedUntil is zero.
	if t.credsRefusedBackoff > 0 && t.acceptedAt().After(t.credsOfferedAt) {
		// The credentials are accepted again, resets the backoff.
		t.log.Infof("the collector accepts node.uuid again")
		t.credsRefusedBackoff = 0
	}
	since := t.unauthorizedSince()
	if since.IsZero() || now.Sub(since) < credsRefusedAfter {
		// not unauthorized, or too early unauthorized
		return
	}
	if t.credsRefusedBackoff == 0 {
		t.credsRefusedBackoff = credsRefusedMin
	} else {
		t.credsRefusedBackoff = min(2*t.credsRefusedBackoff, credsRefusedMax)
	}
	t.credsRefusedUntil = now.Add(t.credsRefusedBackoff)
	t.log.Warnf("node.uuid refused by the collector (401) for %s: the speech goes to the next node able to speak until %s,"+
		" register this node again with 'om node register'", now.Sub(since).Round(time.Second), t.credsRefusedUntil.Format(time.RFC3339))
	t.updateSpeaker()
	t.publishOnChange(t.getState())
}

// resetCredsRefused forgets a refusal of the credentials of the node, as on
// a new node.uuid.
func (t *T) resetCredsRefused() {
	t.credsRefusedUntil = time.Time{}
	t.credsRefusedBackoff = 0
	t.credsOfferedAt = time.Time{}
}
