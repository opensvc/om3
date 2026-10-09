// Package clusterlock takes the cluster locks a client needs, from the daemon
// of its node.
//
// A cluster lock serializes, across the nodes, what reads the state of the
// cluster and changes it on the strength of that reading: two nodes doing it
// at once both read a state the other is about to change. The node speaking
// for the cluster grants every lock, for a lease that frees it should its
// holder die holding it.
//
// A lease is time on a clock, and the nodes and the clients count it on their
// own: a lease passed from one to another goes as the time left, never as an
// expiry, so the clocks need not agree on the time, only tick at the same
// rate. What the lease asks of the holder is to be done before it ends, which
// nothing but the holder can see to: Deadline is when it ends on the clock of
// the holder, and WithDeadline a context ending then, for the work the lock
// protects to stop rather than go on unprotected.
package clusterlock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// Lock is a cluster lock held.
	Lock struct {
		Name string
		ID   string

		// Deadline is when the lease ends, on the clock of the holder, no
		// later than the node speaking ends it.
		Deadline time.Time

		c *client.T
	}

	// Options says how a lock is asked for.
	Options struct {
		// Holder says who holds the lock, for the reader of a listing.
		Holder string

		// Lease is how long the lock is held unless released before. The
		// daemon grants its default when it is zero.
		Lease time.Duration

		// Wait is how long to wait for the lock while another holds it,
		// or while the cluster has no node to grant it.
		Wait time.Duration
	}
)

var (
	// ErrHeld is the error of a lock another still held at the end of the
	// wait.
	ErrHeld = errors.New("cluster lock held")

	// retryInterval is how long a request answered "ask again" waits before
	// it does: the node speaking for the cluster is rebuilding its table, or
	// two nodes disagree on which one speaks, which a moment settles.
	retryInterval = 500 * time.Millisecond

	// returnMargin is what the answer granting a lock is taken to spend on
	// its way back, which the deadline of the holder is brought forward by.
	returnMargin = time.Second
)

// Acquire takes the cluster lock name.
func Acquire(ctx context.Context, name string, opts Options) (*Lock, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	waitUntil := time.Now().Add(opts.Wait)
	for {
		body := api.PostClusterLock{Name: name}
		if opts.Holder != "" {
			body.Holder = &opts.Holder
		}
		if opts.Lease > 0 {
			lease := opts.Lease.String()
			body.Lease = &lease
		}
		if remaining := time.Until(waitUntil); remaining > 0 {
			wait := remaining.String()
			body.Wait = &wait
		}
		sent := time.Now()
		resp, err := c.PostClusterLockWithResponse(ctx, body)
		if err != nil {
			return nil, fmt.Errorf("cluster lock %s: %w", name, err)
		}
		switch {
		case resp.JSON200 != nil:
			return &Lock{Name: name, ID: resp.JSON200.ID, Deadline: deadline(*resp.JSON200, sent, time.Now()), c: c}, nil
		case resp.JSON409 != nil:
			return nil, fmt.Errorf("%w: %s", ErrHeld, resp.JSON409.Detail)
		case resp.StatusCode() == http.StatusServiceUnavailable && time.Until(waitUntil) > retryInterval:
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval):
			}
		case resp.JSON503 != nil:
			return nil, fmt.Errorf("cluster lock %s: %s", name, resp.JSON503.Detail)
		default:
			return nil, fmt.Errorf("cluster lock %s: %s: %s", name, resp.Status(), resp.Body)
		}
	}
}

// deadline returns when the lease of a lock granted ends on the clock of the
// holder, no later than the node speaking ends it.
//
// The lock was granted after the request was sent, so the lease ends no
// sooner than the lease after that. It ends no sooner either than the time
// left when the answer was written, after the answer came back, less what the
// way back took. The later of the two is kept: the first gives up the time
// the request waited for the lock, which the second does not.
func deadline(item api.ClusterLock, sent, received time.Time) time.Time {
	d := sent.Add(item.ExpiresAt.Sub(item.AcquiredAt))
	if item.ExpiresIn != nil {
		if left, err := time.ParseDuration(*item.ExpiresIn); err == nil {
			if t := received.Add(left - returnMargin); t.After(d) {
				d = t
			}
		}
	}
	return d
}

// WithDeadline returns a context ending when the lease does, for the work
// the lock protects, which is to stop rather than go on once another may hold
// the lock.
func (t *Lock) WithDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, t.Deadline)
}

// Release lets the lock go. A lock whose lease ended is not an error to
// release: it is free, which is what releasing it was for.
func (t *Lock) Release(ctx context.Context) error {
	resp, err := t.c.DeleteClusterLockWithResponse(ctx, &api.DeleteClusterLockParams{Name: t.Name, Id: t.ID})
	if err != nil {
		return fmt.Errorf("release cluster lock %s: %w", t.Name, err)
	}
	switch resp.StatusCode() {
	case http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return fmt.Errorf("release cluster lock %s: %s: %s", t.Name, resp.Status(), resp.Body)
	}
}
