package auditstate

import (
	"sync"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	Session struct {
		Q          chan plog.LogMessage
		PreemptC   chan struct{}
		Subsystems []string
		User       string
	}

	Registry struct {
		mu      sync.RWMutex
		active  bool
		current Session
	}
)

func (r *Registry) Start(q chan plog.LogMessage, subsystems []string, preemptC chan struct{}, user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = true
	r.current = Session{
		Q:          q,
		Subsystems: append([]string{}, subsystems...),
		PreemptC:   preemptC,
		User:       user,
	}
}

func (r *Registry) Stop(q chan plog.LogMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return
	}

	if r.current.Q != q {
		return
	}
	r.active = false
	r.current = Session{}
}

func (r *Registry) Snapshot() (Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.active {
		return Session{}, false
	}
	return Session{
		Q:          r.current.Q,
		Subsystems: append([]string{}, r.current.Subsystems...),
		PreemptC:   r.current.PreemptC,
		User:       r.current.User,
	}, true
}

func (r *Registry) Active() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}
