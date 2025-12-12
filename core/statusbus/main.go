// Package statusbus collects and dispatches object rid status changes
//
// It allows:
//
//	Post object rid status
//	Get object rid status
package statusbus

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/status"
)

type (
	register struct {
		path     naming.Path
		rid      string
		hook     func(status.T)
		response chan uuid.UUID
	}

	unregister struct {
		path naming.Path
		rid  string
		uuid uuid.UUID
	}

	postStatus struct {
		path    naming.Path
		rid     string
		state   status.T
		pending bool
	}

	getFirstStatus struct {
		path     naming.Path
		rid      string
		response chan status.T
	}

	getStatus struct {
		path     naming.Path
		rid      string
		response chan status.T
	}

	leaf struct {
		hooks      map[uuid.UUID]func(status.T)
		state      status.T
		firstState status.T
		pending    bool
	}
	statesMap map[naming.Path]map[string]leaf

	T struct {
		states  statesMap
		started bool
		channel struct {
			stop       chan int
			post       chan postStatus
			get        chan any
			register   chan register
			unregister chan unregister
		}
	}

	ObjT struct {
		bus  *T
		path naming.Path
	}

	keyT int
)

const (
	key keyT = 0
)

var (
	ErrorStarted   = errors.New("server already started")
	ErrorNeedStart = errors.New("server not started")
)

// Stop makes the status bus listener stops
func (t *T) Stop() {
	if t.started {
		t.channel.stop <- 1
	}
}

// Start run status bus listener go routine
//
// bus := T{}
// defer bus.Stop()
// bus.Start()
func (t *T) Start() {
	if t.started {
		panic(ErrorStarted)
	}
	t.started = true
	t.channel.stop = make(chan int)
	t.channel.post = make(chan postStatus)
	t.channel.get = make(chan any)
	t.channel.register = make(chan register)
	t.channel.unregister = make(chan unregister)
	t.states = make(statesMap)
	go t.start()
}

// Post push a new object rid status to status bus
//
// bus.Post(naming.Path{Name:"foo",Namespace: "ns1", Kind: kind.Svc},
//
//	"app#1",
//	status.Down))
func (t *T) Post(p naming.Path, rid string, state status.T, pending bool) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.channel.post <- postStatus{
		path:    p,
		rid:     rid,
		state:   state,
		pending: pending,
	}
}

func (t *T) Pending(p naming.Path, rid string) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.channel.post <- postStatus{
		path:    p,
		rid:     rid,
		state:   -1,
		pending: true,
	}
}

// Get retrieve an object rid status
//
// returns status.Undef if no object rid is not found
//
// Example:
//
//	p := path.Path{Name:"foo",Namespace: "ns1", Kind: kind.Svc}
//	bus.Post(p, "app#1", status.Up)
//	bus.Post(p, "app#2", status.Down)
//
//	bus.Get(p, "app#1") // returns status.Up
//	bus.Get(p, "app#2") // returns status.Down
//	bus.Get(p, "app#99") // returns status.Undef
//	bus.Get(path.Path{}, "app#1") // returns status.Undef
func (t *T) Get(p naming.Path, rid string) status.T {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan status.T)
	t.channel.get <- getStatus{
		path:     p,
		rid:      rid,
		response: resp,
	}
	return <-resp
}

func (t *T) First(p naming.Path, rid string) status.T {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan status.T)
	t.channel.get <- getFirstStatus{
		path:     p,
		rid:      rid,
		response: resp,
	}
	return <-resp
}

func (t *T) Wait(p naming.Path, rid string, timeout time.Duration) status.T {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan status.T)
	u := t.Register(p, rid, func(s status.T) {
		resp <- s
	})
	defer t.Unregister(p, rid, u)
	if timeout == 0 {
		return <-resp
	}
	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()
	select {
	case s := <-resp:
		return s
	case <-timer.C:
		return status.Undef
	}
}

func (t *T) Register(p naming.Path, rid string, hook func(status.T)) uuid.UUID {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan uuid.UUID)
	t.channel.register <- register{
		path:     p,
		rid:      rid,
		hook:     hook,
		response: resp,
	}
	return <-resp
}

func (t *T) Unregister(p naming.Path, rid string, u uuid.UUID) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.channel.unregister <- unregister{
		path: p,
		rid:  rid,
		uuid: u,
	}
}

func (t T) getLeaf(p naming.Path, rid string) leaf {
	if m, ok := t.states[p]; ok {
		if l, ok := m[rid]; ok {
			return l
		}
	}
	return leaf{state: status.Undef}
}

func (t *T) addLeaf(p naming.Path, rid string) {
	_, ok := t.states[p]
	if !ok {
		t.states[p] = make(map[string]leaf)
	}
	_, ok = t.states[p][rid]
	if !ok {
		t.states[p][rid] = leaf{
			state:   status.Undef,
			pending: true,
			hooks:   make(map[uuid.UUID]func(status.T)),
		}
	}
}

func (t *T) delHook(p naming.Path, rid string, u uuid.UUID) {
	l := t.getLeaf(p, rid)
	if _, ok := l.hooks[u]; ok {
		delete(t.states[p][rid].hooks, u)
	}
}

func (t *T) addHook(p naming.Path, rid string, hook func(status.T)) uuid.UUID {
	t.addLeaf(p, rid)
	u := uuid.New()
	t.states[p][rid].hooks[u] = hook
	return u

}

func (t *T) post(p naming.Path, rid string, state status.T, pending bool) {
	t.addLeaf(p, rid)
	l := t.getLeaf(p, rid)
	l.pending = pending
	if state >= 0 {
		l.state = state
	}
	if l.firstState == 0 {
		l.firstState = state
	}
	t.states[p][rid] = l
	if !l.pending {
		for _, hook := range l.hooks {
			hook(l.state)
		}
	}
}

func (t *T) start() {
	for {
		select {
		case <-t.channel.stop:
			return
		case req := <-t.channel.register:
			req.response <- t.addHook(req.path, req.rid, req.hook)
		case req := <-t.channel.unregister:
			t.delHook(req.path, req.rid, req.uuid)
		case req := <-t.channel.post:
			t.post(req.path, req.rid, req.state, req.pending)
		case i := <-t.channel.get:
			switch req := i.(type) {
			case getStatus:
				req.response <- t.getLeaf(req.path, req.rid).state
			case getFirstStatus:
				req.response <- t.getLeaf(req.path, req.rid).firstState
			}
		}
	}
}

func (t *ObjT) Wait(rid string, timeout time.Duration) status.T {
	return t.bus.Wait(t.path, rid, timeout)
}

func (t *ObjT) First(rid string) status.T {
	return t.bus.First(t.path, rid)
}

func (t *ObjT) Get(rid string) status.T {
	return t.bus.Get(t.path, rid)
}

func (t *ObjT) Pending(rid string) {
	t.bus.Pending(t.path, rid)
}

func (t *ObjT) Post(rid string, state status.T, pending bool) {
	t.bus.Post(t.path, rid, state, pending)
}

func (t *ObjT) Register(rid string, hook func(status.T)) uuid.UUID {
	return t.bus.Register(t.path, rid, hook)
}

func (t *ObjT) Unregister(rid string, u uuid.UUID) {
	t.bus.Unregister(t.path, rid, u)
}

func NewObjectBus(p naming.Path) *ObjT {
	t := ObjT{
		path: p,
		bus:  &T{},
	}
	return &t
}

func WithContext(ctx context.Context, p naming.Path) (context.Context, func()) {
	if sb := FromContext(ctx); sb != nil {
		// the context already has a statusbus
		return ctx, func() {}
	}
	sb := NewObjectBus(p)
	sb.bus.Start()
	newCtx := context.WithValue(ctx, key, sb)
	stopper := func() { sb.bus.Stop() }
	return newCtx, stopper
}

func FromContext(ctx context.Context) *ObjT {
	obj, ok := ctx.Value(key).(*ObjT)
	if !ok {
		return nil
	}
	return obj
}
