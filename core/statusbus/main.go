// Package statusbus provide bus maintain object rid status
//
// It allows:
//    Post object rid status
//    Get object rid status
//

package statusbus

import (
	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
)

type (
	postStatus struct {
		objectPath path.T
		rid        string
		state      status.T
	}

	getStatus struct {
		objectPath path.T
		rid        string
		response   chan status.T
	}

	statesMap map[path.T]map[string]status.T

	T struct {
		states  statesMap
		started bool
		stop    chan int
		post    chan postStatus
		get     chan getStatus
	}
)

var (
	ErrorStarted   = errors.New("server already started")
	ErrorNeedStart = errors.New("server not started")
)

// Stop makes the status bus listener stops
func (t *T) Stop() {
	if t.started {
		t.stop <- 1
	}
}

// Start run status bus listener go routine
//
// bus := T{}
// defer bus.Stop()
// bus.Start()
//
func (t *T) Start() {
	if t.started {
		panic(ErrorStarted)
	}
	t.started = true
	t.stop = make(chan int)
	t.post = make(chan postStatus)
	t.get = make(chan getStatus)
	t.states = make(statesMap)
	go t.start()
}

// Post push a new object rid status to status bus
//
// bus.Post(path.T{Name:"foo",Namespace: "root", Kind: kind.Svc},
//          "app#1",
//          status.Down))
//
func (t *T) Post(p path.T, rid string, status status.T) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.post <- postStatus{
		objectPath: p,
		rid:        rid,
		state:      status,
	}
}

// Get retrieve an object rid status
//
// returns status.Undef if no object rid is not found
//
// Example:
//    p := path.T{Name:"foo",Namespace: "root", Kind: kind.Svc}
//    bus.Post(p, "app#1", status.Up)
//    bus.Post(p, "app#2", status.Down)
//
//    bus.Get(p, "app#1") // returns status.Up
//    bus.Get(p, "app#2") // returns status.Down
//    bus.Get(p, "app#99") // returns status.Undef
//    bus.Get(path.T{}, "app#1") // returns status.Undef
//
func (t *T) Get(p path.T, rid string) status.T {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan status.T)
	t.get <- getStatus{
		objectPath: p,
		rid:        rid,
		response:   resp,
	}
	return <-resp
}

func (t *T) start() {
	for {
		select {
		case <-t.stop:
			return
		case req := <-t.post:
			if m, ok := t.states[req.objectPath]; ok {
				m[req.rid] = req.state
			} else {
				t.states[req.objectPath] = map[string]status.T{req.rid: req.state}
			}
		case req := <-t.get:
			resp := req.response
			if m, ok := t.states[req.objectPath]; ok {
				if state, ok := m[req.rid]; ok {
					resp <- state
					continue
				}
			}
			resp <- status.Undef
		}
	}
}
