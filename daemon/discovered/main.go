// Package discovered says whether the daemon has discovered the objects of
// the configuration files it found as it started.
//
// The api lists the objects from the status data the object monitors fill,
// which they do a while after the daemon serves its api: an object listing
// answered empty then is no answer, and a client taking it for one reports
// existing objects as not found (card 308).
package discovered

import "sync/atomic"

var settled atomic.Bool

// Settled returns true once the objects of the configuration files found as
// the daemon started are discovered, or the discovery gave up waiting.
func Settled() bool {
	return settled.Load()
}

// SetSettled marks the initial discovery done.
func SetSettled() {
	settled.Store(true)
}

// Reset marks the initial discovery not done, as a daemon starting.
func Reset() {
	settled.Store(false)
}
