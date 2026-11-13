package evwatch

import (
	"sync"
	"time"
)

// Debouncer calls a function for a key once the notifications about it
// settle: a notification within the delay of the previous one delays the
// call, so the events of one change, as the two lines of a domain start, make
// one call.
type Debouncer struct {
	mu      sync.Mutex
	delay   time.Duration
	fn      func(key string)
	timers  map[string]*time.Timer
	stopped bool
}

// NewDebouncer returns a Debouncer calling fn for a key delay after the last
// notification about it.
func NewDebouncer(delay time.Duration, fn func(key string)) *Debouncer {
	return &Debouncer{
		delay:  delay,
		fn:     fn,
		timers: make(map[string]*time.Timer),
	}
}

// Notify notifies a change of key.
func (t *Debouncer) Notify(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return
	}
	if timer, ok := t.timers[key]; ok {
		timer.Reset(t.delay)
		return
	}
	t.timers[key] = time.AfterFunc(t.delay, func() {
		t.mu.Lock()
		delete(t.timers, key)
		stopped := t.stopped
		t.mu.Unlock()
		if !stopped {
			t.fn(key)
		}
	})
}

// Stop drops the calls pending, and the notifications to come.
func (t *Debouncer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	for key, timer := range t.timers {
		timer.Stop()
		delete(t.timers, key)
	}
}
