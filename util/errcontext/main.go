// Package errcontext defines a context-aware error handling mechanism using
// interfaces and a struct that allows sending and receiving a single error.
package errcontext

import (
	"context"
	"fmt"
	"sync"
)

type (
	// ErrCloser defines the method to close an error
	ErrCloser interface {
		Close()
	}

	// ErrReceiver defines the method to receive an error.
	ErrReceiver interface {
		Receive() error
	}

	// ErrSender defines the method to send an error.
	ErrSender interface {
		Send(error)
	}

	// ErrCloseSender embeds ErrCloser and ErrSender interfaces.
	ErrCloseSender interface {
		ErrCloser
		ErrSender
	}

	// ErrContexer embeds ErrReceiver, ErrSender and ErrCloser interfaces.
	ErrContexer interface {
		ErrCloser
		ErrReceiver
		ErrSender
	}

	// errCtx implements the ErrContexer interface.
	errCtx struct {
		// errC is a buffered channel to hold a single error
		errC chan error

		// ctx is a context to manage cancellation and deadlines.
		ctx context.Context

		// mu is a mutex to synchronize access to closed, errSent and receiveCalled
		mu sync.Mutex

		// closed is a flag indicating if the error channel has been closed.
		closed bool

		// receiveCalled is a flag preventing multiple Receive calls.
		receiveCalled bool

		// errSent is a flag flag indicating if an error has been sent.
		errSent bool
	}
)

var (
	ErrAlreadyCalled = fmt.Errorf("already called")
)

// New creates ErrContexer with an error channel and context.
// When used ctx is nil, background context is used => Receive() calls are
// blocking until Send call or Close call.
func New(ctx context.Context) ErrContexer {
	if ctx == nil {
		ctx = context.Background()
	}
	return &errCtx{errC: make(chan error, 1), ctx: ctx}
}

// Send Sends an error if one has not been sent yet and if the context is still active.
func (e *errCtx) Send(err error) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	if e.errSent {
		return
	}
	e.errSent = true
	select {
	case <-e.ctx.Done():
		// Context to send error is done, drop error,
		// the errCtx.Receive will return context error.
	case e.errC <- err:
	}
}

// Receive returns:
//
//	the received error from the channel
//	or the context error if the context is done
//	or ErrAlreadyCalled if already called
func (e *errCtx) Receive() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	if e.receiveCalled {
		e.mu.Unlock()
		return ErrAlreadyCalled
	} else {
		e.receiveCalled = true
		e.mu.Unlock()
	}
	select {
	case err := <-e.errC:
		return err
	case <-e.ctx.Done():
		return e.ctx.Err()
	}
}

// Close closes the error channel to signal that no more errors will be sent.
func (e *errCtx) Close() {
	if e != nil {
		e.mu.Lock()
		defer e.mu.Unlock()
		if !e.closed {
			close(e.errC)
			e.closed = true
		}
	}
}
