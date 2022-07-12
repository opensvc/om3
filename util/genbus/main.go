// Package genbus provide bus maintain struct associated with ids
//
// It allows:
//    Post object id status
//    Get object id status
//

package genbus

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type (
	TName string

	THook struct {
		fn   func(interface{})
		once bool
	}

	register struct {
		name     TName
		rid      string
		hook     THook
		response chan uuid.UUID
		once     bool
	}

	unregister struct {
		name TName
		rid  string
		uuid uuid.UUID
	}

	postI struct {
		name    TName
		id      string
		i       interface{}
		pending bool
	}

	getI struct {
		name TName
		id   string
		i    chan interface{}
	}

	leaf struct {
		hooks   map[uuid.UUID]THook
		i       interface{}
		pending bool
	}
	objectMap map[TName]map[string]leaf

	T struct {
		objects objectMap
		started bool
		channel struct {
			stop       chan int
			post       chan postI
			get        chan getI
			register   chan register
			unregister chan unregister
		}
		checker Checker
	}

	Checker interface {
		Valid() bool
	}

	ObjT struct {
		bus  *T
		name TName
	}

	keyT int
)

const (
	key keyT = iota
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
//
func (t *T) Start() {
	if t.started {
		panic(ErrorStarted)
	}
	t.started = true
	t.channel.stop = make(chan int)
	t.channel.post = make(chan postI)
	t.channel.get = make(chan getI)
	t.channel.register = make(chan register)
	t.channel.unregister = make(chan unregister)
	t.objects = make(objectMap)
	go t.start()
}

// Post push a new object id i to bus
//
// bus.Post(TNameT("foo"),
//          "idx",
//          i))
//
func (t *T) Post(p TName, id string, i interface{}, pending bool) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.channel.post <- postI{
		name:    p,
		id:      id,
		i:       i,
		pending: pending,
	}
}

func (t *T) Pending(p TName, id string) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.channel.post <- postI{
		name:    p,
		id:      id,
		i:       nil,
		pending: true,
	}
}

// Get retrieve object with id
//
// returns status.Undef if no object id is not found
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
func (t *T) Get(name TName, id string) interface{} {
	if !t.started {
		panic(ErrorNeedStart)
	}
	i := make(chan interface{})
	t.channel.get <- getI{
		name: name,
		id:   id,
		i:    i,
	}
	return <-i
}

func (t *T) Wait(name TName, id string, timeout time.Duration) interface{} {
	if !t.started {
		panic(ErrorNeedStart)
	}
	iC := make(chan interface{})
	u := t.Register(name, id, func(s interface{}) {
		iC <- s
	})
	defer t.Unregister(name, id, u)
	if timeout == 0 {
		return <-iC
	}
	timer := time.NewTimer(timeout)
	select {
	case s := <-iC:
		if !timer.Stop() {
			<-timer.C
		}
		return s
	case <-timer.C:
		return nil
	}
}

func (t *T) Register(p TName, id string, fn func(interface{})) uuid.UUID {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan uuid.UUID)
	t.channel.register <- register{
		name: p,
		rid:  id,
		hook: THook{
			fn: fn,
		},
		response: resp,
	}
	return <-resp
}

func (t *T) RegisterOnce(p TName, id string, fn func(interface{})) uuid.UUID {
	if !t.started {
		panic(ErrorNeedStart)
	}
	resp := make(chan uuid.UUID)
	t.channel.register <- register{
		name: p,
		rid:  id,
		hook: THook{
			fn:   fn,
			once: true,
		},
		response: resp,
	}
	return <-resp
}

func (t *T) Unregister(name TName, id string, u uuid.UUID) {
	if !t.started {
		panic(ErrorNeedStart)
	}
	t.channel.unregister <- unregister{
		name: name,
		rid:  id,
		uuid: u,
	}
}

func (t T) getLeaf(name TName, id string) leaf {
	if m, ok := t.objects[name]; ok {
		if l, ok := m[id]; ok {
			return l
		}
	}
	return leaf{i: nil}
}

func (t *T) addLeaf(name TName, id string) {
	_, ok := t.objects[name]
	if !ok {
		t.objects[name] = make(map[string]leaf)
	}
	_, ok = t.objects[name][id]
	if !ok {
		t.objects[name][id] = leaf{
			i:       nil,
			pending: true,
			hooks:   make(map[uuid.UUID]THook),
		}
	}
}

func (t *T) delHook(p TName, id string, u uuid.UUID) {
	l := t.getLeaf(p, id)
	if _, ok := l.hooks[u]; ok {
		delete(t.objects[p][id].hooks, u)
	}
}

func (t *T) addHook(name TName, id string, hook THook) uuid.UUID {
	t.addLeaf(name, id)
	u := uuid.New()
	t.objects[name][id].hooks[u] = hook
	return u
}

func (t *T) post(name TName, id string, i interface{}, pending bool) {
	t.addLeaf(name, "*")
	t.addLeaf(name, id)
	l := t.getLeaf(name, id)
	lAll := t.getLeaf(name, "*")
	l.pending = pending
	if i != nil {
		l.i = i
	}
	t.objects[name]["*"] = l
	t.objects[name][id] = l
	if !l.pending {
		for id, hook := range lAll.hooks {
			go hook.fn(l.i)
			if hook.once {
				delete(l.hooks, id)
			}
		}
		for id, hook := range l.hooks {
			go hook.fn(l.i)
			if hook.once {
				delete(l.hooks, id)
			}
		}
	}
}

func (t *T) start() {
	for {
		select {
		case <-t.channel.stop:
			return
		case req := <-t.channel.register:
			req.response <- t.addHook(req.name, req.rid, req.hook)
		case req := <-t.channel.unregister:
			t.delHook(req.name, req.rid, req.uuid)
		case req := <-t.channel.post:
			t.post(req.name, req.id, req.i, req.pending)
		case req := <-t.channel.get:
			req.i <- t.getLeaf(req.name, req.id).i
		}
	}
}

func (t *ObjT) Wait(id string, timeout time.Duration) interface{} {
	return t.bus.Wait(t.name, id, timeout)
}

func (t *ObjT) Get(id string) interface{} {
	return t.bus.Get(t.name, id)
}

func (t *ObjT) Pending(id string) {
	t.bus.Pending(t.name, id)
}

func (t *ObjT) Post(id string, i interface{}, pending bool) {
	t.bus.Post(t.name, id, i, pending)
}

func (t *ObjT) Register(id string, hook func(interface{})) uuid.UUID {
	return t.bus.Register(t.name, id, hook)
}

func (t *ObjT) RegisterOnce(id string, hook func(interface{})) uuid.UUID {
	return t.bus.RegisterOnce(t.name, id, hook)
}

func (t *ObjT) Unregister(id string, u uuid.UUID) {
	t.bus.Unregister(t.name, id, u)
}

func NewObjectBus(name TName) *ObjT {
	t := ObjT{
		name: name,
		bus:  &T{},
	}
	return &t
}

func WithContext(ctx context.Context, name TName) (context.Context, func()) {
	sb := NewObjectBus(name)
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

func WithGlobal(ctx context.Context) (context.Context, func()) {
	sb := &T{}
	sb.Start()
	newCtx := context.WithValue(ctx, "bus:global", sb)
	stopper := func() { sb.Stop() }
	return newCtx, stopper
}

func NameBus(ctx context.Context, name TName) *ObjT {
	bus, ok := ctx.Value("bus:global").(*T)
	if !ok {
		return nil
	}
	return &ObjT{
		bus:  bus,
		name: name,
	}
}
