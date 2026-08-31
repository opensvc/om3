package tui

import (
	"io"
	"sync"
)

type AtomicCloserSlice struct {
	mu      sync.RWMutex
	closers []io.Closer

	// closed is set by CloseAll and cleared by Reset. It makes Append reject
	// the closers opened by a goroutine that lost the race with CloseAll.
	closed bool
}

// Append adds a closer to the slice thread-safely. It returns false, and does
// not take ownership of c, when CloseAll was called since the last Reset: the
// caller is expected to close c itself.
func (a *AtomicCloserSlice) Append(c io.Closer) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return false
	}
	a.closers = append(a.closers, c)
	return true
}

// Reset re-arms the slice: Append accepts closers again.
func (a *AtomicCloserSlice) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = false
}

// Get returns the slice thread-safely.
func (a *AtomicCloserSlice) Get() []io.Closer {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Return a copy to avoid race conditions on the underlying array.
	closers := make([]io.Closer, len(a.closers))
	copy(closers, a.closers)
	return closers
}

// CloseAll closes all closers in the slice thread-safely.
func (a *AtomicCloserSlice) CloseAll() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var errs []error
	for _, c := range a.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	a.closers = nil // Clear the slice after closing.
	a.closed = true
	if len(errs) > 0 {
		return errs[0] // Or use errors.Join(errs...) in Go 1.20+
	}
	return nil
}

// Len returns the length of the slice thread-safely.
func (a *AtomicCloserSlice) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.closers)
}
