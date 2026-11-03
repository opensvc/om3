// Package signalrelay relays a signal the process receives to the commands it
// runs, rather than letting it end the process and leave them running.
//
// A command run through om exec execve's into what it runs, in a process
// group and a cgroup of its own: a signal ending om, as the kill of an exec
// by the daemon sends, orphans it, and it runs on. With a relay, om passes
// the first signal on to the commands subscribed to it, waits for them to
// end, and reports how they ended. A second signal ends om as if not caught.
package signalrelay

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

type (
	relay struct {
		mu       sync.Mutex
		received os.Signal
		subs     map[chan os.Signal]struct{}
	}

	key struct{}
)

// WithSignals returns a context carrying a relay of the first of sigs the
// process receives, and the function to call when the relay is no longer
// needed. While the relay holds, these signals no longer end the process.
func WithSignals(ctx context.Context, sigs ...os.Signal) (context.Context, func()) {
	r := &relay{subs: make(map[chan os.Signal]struct{})}
	c := make(chan os.Signal, 1)
	signal.Notify(c, sigs...)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-c:
			// the next one ends the process.
			signal.Reset(sigs...)
			r.broadcast(sig)
		case <-done:
		}
	}()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			signal.Stop(c)
			close(done)
		})
	}
	return context.WithValue(ctx, key{}, r), stop
}

func (r *relay) broadcast(sig os.Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = sig
	for c := range r.subs {
		select {
		case c <- sig:
		default:
		}
	}
}

// Subscribe returns the channel the signal the relay of ctx receives is
// sent on, and the function to unsubscribe. A signal received before the
// subscription is sent at once: a command started after it, as the one of a
// task after its install, is signaled too. With no relay in ctx, the channel
// is nil.
func Subscribe(ctx context.Context) (<-chan os.Signal, func()) {
	r, ok := ctx.Value(key{}).(*relay)
	if !ok || r == nil {
		return nil, func() {}
	}
	c := make(chan os.Signal, 1)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.received != nil {
		c <- r.received
	}
	r.subs[c] = struct{}{}
	return c, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.subs, c)
	}
}

// Received returns the signal the relay of ctx received, if any.
func Received(ctx context.Context) os.Signal {
	r, ok := ctx.Value(key{}).(*relay)
	if !ok || r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.received
}
