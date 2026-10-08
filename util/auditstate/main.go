package auditstate

import (
	"sync"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	Session struct {
		Q chan plog.LogMessage

		// PreemptC is closed when another session preempts this one.
		PreemptC   chan struct{}
		Subsystems []string
		User       string
	}

	Registry struct {
		mu      sync.RWMutex
		active  bool
		current Session

		// beginMu orders the sessions beginning: a session is registered
		// and activated before the next one begins. It is not mu, so the
		// subsystems reading the registry while a session activates are
		// not blocked.
		beginMu sync.Mutex
	}
)

// Begin makes a session the current one, and says whether it did: there is
// at most one. A current session is returned and kept, unless preempt asks
// to end it: it is then told so, by the close of its PreemptC, and replaced.
//
// The check and the replacement are one step, so two sessions beginning at
// once can not both find none.
//
// activate, when not nil, is called for a session that begins, before the
// next session can begin: it tells the subsystems to send their logs to q.
// A session preempted right after it registered has then told them before
// the session preempting it, which they hear last and keep. The stop of the
// preempted session, naming a queue they no longer send to, changes nothing.
func (r *Registry) Begin(q chan plog.LogMessage, subsystems []string, preemptC chan struct{}, user string, preempt bool, activate func()) (Session, bool) {
	r.beginMu.Lock()
	defer r.beginMu.Unlock()
	sess, ok := r.register(q, subsystems, preemptC, user, preempt)
	if ok && activate != nil {
		activate()
	}
	return sess, ok
}

func (r *Registry) register(q chan plog.LogMessage, subsystems []string, preemptC chan struct{}, user string, preempt bool) (Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active {
		if !preempt {
			return r.current, false
		}
		close(r.current.PreemptC)
	}
	r.active = true
	r.current = Session{
		Q:          q,
		Subsystems: append([]string{}, subsystems...),
		PreemptC:   preemptC,
		User:       user,
	}
	return r.current, true
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
