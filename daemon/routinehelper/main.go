/*
Package routinehelper provides counter for goroutines

Example:
	type (
		T struct {
			routinehelper.TT
		}
	)

	func New() *T {
		return &T{TT: *routinehelper.NewTracer()}
	}

	func (t *T) fooRoutineLoop(nb int) {
		done := make(chan bool)
		running := make(chan bool)
		for i := 0; i < nb; i++ {
			go func() {
				defer t.Trace("foo")()
				// foo routine code
			}()
			go func() {
				defer t.Trace("bar")()
				// bar routine code
			}()
		}
	}

    func (t *T) ShowStats() routinehelper.Stat {
        return t.TraceRDump()
    }

*/
package routinehelper

import (
	"sync"
)

type (
	// TT struct holds a tracer
	TT struct {
		t Tracer
	}
	Tracer interface {
		Trace(string) func()
		TraceRDump() Stat
	}

	trace struct {
		count        int
		countDetails map[string]int
		countMax     int
		lock         *sync.RWMutex
	}

	noopTrace struct {
	}

	Stat struct {
		Count   int
		Max     int
		Details map[string]int
	}
)

// SetTracer may be used in funcopts to set tracer
func (tt *TT) SetTracer(i Tracer) {
	tt.t = i
}

// Trace increments routines calls associated with name, and returns
// func that will decrement routines calls associated with name
//
// Example:
//	go func() {
//     defer Trace("routine-name")()
//     ....
//  }
func (tt *TT) Trace(s string) func() {
	return tt.t.Trace(s)
}

// TraceRDump() returns stats about traced routine calls
func (tt *TT) TraceRDump() Stat {
	return tt.t.TraceRDump()
}

// NewTracer() create a new tracer
func NewTracer() *TT {
	return &TT{
		t: &trace{
			lock:         &sync.RWMutex{},
			countDetails: make(map[string]int),
		},
	}
}

// NewTracer() create a noop tracer
func NewTracerNoop() *TT {
	return &TT{&noopTrace{}}
}

func (t *trace) Trace(name string) func() {
	t.inc(name)
	return func() {
		t.dec(name)
	}
}

func (t *noopTrace) Trace(_ string) func() {
	return func() {}
}

func (t *noopTrace) TraceRDump() Stat {
	return Stat{}
}

func (t *trace) TraceRDump() Stat {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return Stat{
		Count:   t.count,
		Max:     t.countMax,
		Details: t.values(),
	}
}

func (t *trace) Value() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.count
}

func (t *trace) values() map[string]int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	newMap := make(map[string]int)
	for key, value := range t.countDetails {
		newMap[key] = value
	}
	return newMap
}

func (t *trace) Max() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.countMax
}

func (t *trace) inc(name string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.count = t.count + 1
	if _, ok := t.countDetails[name]; ok {
		t.countDetails[name] = t.countDetails[name] + 1
	} else {
		t.countDetails[name] = 1
	}
	if t.count > t.countMax {
		t.countMax = t.count
	}
}

func (t *trace) dec(name string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.count = t.count - 1
	t.countDetails[name] = t.countDetails[name] - 1
}
