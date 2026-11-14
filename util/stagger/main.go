// Package stagger connects to the first of several addresses to accept the
// connection, without the one that does not answer holding the others: the
// connect to an address starts when the one to the previous address failed,
// or ran a delay, as a dual-stack client falls back from an address of a
// family the node can not reach.
package stagger

import (
	"context"
	"net"
	"time"
)

// Attempt connects to one address.
type Attempt func(ctx context.Context) (net.Conn, error)

// Dial runs the attempts in order, each one starting when the previous one
// failed or ran delay, and returns the first connection established. The
// attempts still running are abandoned then: their context ends, and a
// connection they establish meanwhile is closed. It returns the error of the
// first attempt when none connects.
func Dial(ctx context.Context, delay time.Duration, attempts []Attempt) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	if len(attempts) == 0 {
		return nil, &net.AddrError{Err: "no address to connect to"}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(attempts))
	next, pending := 0, 0
	start := func() {
		attempt := attempts[next]
		next++
		pending++
		go func() {
			c, err := attempt(ctx)
			results <- result{c, err}
		}()
	}
	start()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	var firstErr error
	for pending > 0 {
		var timerC <-chan time.Time
		if next < len(attempts) {
			timerC = timer.C
		}
		select {
		case <-timerC:
			start()
			timer.Reset(delay)
		case r := <-results:
			pending--
			if r.err == nil {
				go func(n int) {
					for ; n > 0; n-- {
						if late := <-results; late.conn != nil {
							_ = late.conn.Close()
						}
					}
				}(pending)
				return r.conn, nil
			}
			if firstErr == nil {
				firstErr = r.err
			}
			if next < len(attempts) && ctx.Err() == nil {
				start()
				timer.Reset(delay)
			}
		}
	}
	return nil, firstErr
}
