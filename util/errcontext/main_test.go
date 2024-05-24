package errcontext

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOnNilErrCtx(t *testing.T) {
	var i *errCtx
	t.Run("Send is not blocking", func(t *testing.T) {
		require.NotPanics(t, func() { i.Send(nil) })
		require.NotPanicsf(t, func() { i.Send(nil) }, "multiple calls")
	})
	t.Run("Receive is not bloking", func(t *testing.T) {
		var err error
		require.NotPanics(t, func() {
			err = i.Receive()
		})
		require.Nilf(t, err, "Receive() result should be nil")
		require.Nilf(t, i.Receive(), "multiple calls")
	})
}

func TestErrCtxWithValidContext(t *testing.T) {
	t.Run("Receive", func(t *testing.T) {
		t.Run("respect cancellation", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			go func() {
				time.Sleep(5 * time.Millisecond)
				cancel()
			}()
			e := New(ctx)
			defer e.Close()
			require.ErrorIs(t, e.Receive(), context.Canceled)
		})

		t.Run("respect timeout", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			e := New(ctx)
			defer e.Close()
			require.ErrorIs(t, e.Receive(), context.DeadlineExceeded)
		})

		t.Run("multiple Send call are dropped", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			e := New(ctx)
			defer e.Close()
			err1 := fmt.Errorf("err1")
			err2 := fmt.Errorf("err2")
			e.Send(err1)
			e.Send(err2)
			require.ErrorIs(t, e.Receive(), err1)
			require.ErrorIs(t, e.Receive(), ErrAlreadyCalled)
		})
	})
}

func TestWithErrCtxCreatedFromNilContextAreBlocking(t *testing.T) {
	maxDuration := 10 * time.Millisecond

	t.Run("Receive is blocked until Close is called", func(t *testing.T) {
		e := New(nil)
		defer e.Close()
		go func() {
			time.Sleep(maxDuration / 2)
			t.Logf("closing")
			e.Close()
		}()

		errC := make(chan error)
		go func() {
			t.Logf("receiving")
			err := e.Receive()
			t.Logf("received")
			errC <- err
		}()
		select {
		case <-time.After(maxDuration):
			t.Errorf("Receive() didn't return even after errContext is Closed")
		case err := <-errC:
			t.Logf("Received ok")
			require.Nil(t, err, "received error should be nil")
			require.ErrorIs(t, e.Receive(), ErrAlreadyCalled)
		}
	})

	t.Run("Receive is blocked until Send is called", func(t *testing.T) {
		e := New(nil)
		defer e.Close()
		expectedError := fmt.Errorf("expected error XX")
		go func() {
			time.Sleep(maxDuration / 2)
			t.Logf("closing")
			e.Send(expectedError)
		}()

		errC := make(chan error)
		go func() {
			t.Logf("receiving")
			err := e.Receive()
			t.Logf("received")
			errC <- err
		}()
		select {
		case <-time.After(maxDuration):
			t.Errorf("Receive() didn't return even after Send is called")
		case err := <-errC:
			t.Logf("Received ok")
			require.ErrorIs(t, err, expectedError)
			require.ErrorIs(t, e.Receive(), ErrAlreadyCalled)
			t.Run("multiple Send are not bloking after Receive", func(t *testing.T) {
				e.Send(nil)
			})
		}
	})
}
