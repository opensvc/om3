package tui

import (
	"io"
	"sync"
)

type AtomicCloserSlice struct {
	mu      sync.RWMutex
	closers []io.Closer
}

// Append adds a closer to the slice thread-safely.
func (a *AtomicCloserSlice) Append(c io.Closer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closers = append(a.closers, c)
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
