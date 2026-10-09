// Package clusterlock takes the cluster locks a client needs, from the daemon
// of its node.
//
// A cluster lock serializes, across the nodes, what reads the state of the
// cluster and changes it on the strength of that reading: two nodes doing it
// at once both read a state the other is about to change. The node speaking
// for the cluster grants every lock, for a lease that frees it should its
// holder die holding it.
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
		Name      string
		ID        string
		ExpiresAt time.Time

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
)

// Acquire takes the cluster lock name.
func Acquire(ctx context.Context, name string, opts Options) (*Lock, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(opts.Wait)
	for {
		body := api.PostClusterLock{Name: name}
		if opts.Holder != "" {
			body.Holder = &opts.Holder
		}
		if opts.Lease > 0 {
			lease := opts.Lease.String()
			body.Lease = &lease
		}
		if remaining := time.Until(deadline); remaining > 0 {
			wait := remaining.String()
			body.Wait = &wait
		}
		resp, err := c.PostClusterLockWithResponse(ctx, body)
		if err != nil {
			return nil, fmt.Errorf("cluster lock %s: %w", name, err)
		}
		switch {
		case resp.JSON200 != nil:
			return &Lock{Name: name, ID: resp.JSON200.ID, ExpiresAt: resp.JSON200.ExpiresAt, c: c}, nil
		case resp.JSON409 != nil:
			return nil, fmt.Errorf("%w: %s", ErrHeld, resp.JSON409.Detail)
		case resp.StatusCode() == http.StatusServiceUnavailable && time.Until(deadline) > retryInterval:
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
