// Package session remembers the actions the daemon ran, so a client that
// submitted one can ask how it ended.
//
// A client submitting an action is handed a session id, and an orchestrated
// action an orchestration id, which several sessions run under. Neither is
// answerable from the process table: that one is keyed by pid, holds only
// what is running, and exists to be signaled. This one is keyed by the ids
// the client was given, and outlives the process.
//
// What it holds is bounded, and the bound is the point: a session nobody
// asked about is forgotten rather than kept forever. A client asking for one
// that has been forgotten is told so, which is not the same answer as never
// having heard of it, so a poller can tell "not finished" from "I no longer
// know".
package session

import (
	"sort"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	// State is where a session got to.
	State string

	// Session is one exec the daemon ran, or is running.
	//
	// The exec is the unit, not the session: one command reaches several
	// objects of a node under one session id, each of them its own exec with
	// its own outcome and its own duration. So the exec id is what a record
	// is keyed by, and the session id is what several of them share.
	Session struct {
		SessionID       string        `json:"session_id"`
		ExecID          string        `json:"exec_id"`
		OrchestrationID string        `json:"orchestration_id,omitempty"`
		Node            string        `json:"node"`
		Path            string        `json:"path,omitempty"`
		Origin          string        `json:"origin"`
		Title           string        `json:"title,omitempty"`
		Command         string        `json:"command"`
		State           State         `json:"state"`
		Error           string        `json:"error,omitempty"`
		BeginAt         time.Time     `json:"begin_at"`
		EndAt           *time.Time    `json:"end_at,omitempty"`
		Duration        time.Duration `json:"duration,omitempty"`
	}

	// Orchestration is one orchestration the daemon accepted, and the
	// sessions it ran under it.
	Orchestration struct {
		OrchestrationID string     `json:"orchestration_id"`
		Node            string     `json:"node"`
		Path            string     `json:"path,omitempty"`
		GlobalExpect    string     `json:"global_expect,omitempty"`
		State           State      `json:"state"`
		Error           string     `json:"error,omitempty"`
		BeginAt         time.Time  `json:"begin_at"`
		EndAt           *time.Time `json:"end_at,omitempty"`
	}
)

const (
	// StateRunning is a session the daemon has not seen the end of.
	StateRunning State = "running"

	// StateSucceeded is a session that ended without an error.
	StateSucceeded State = "succeeded"

	// StateFailed is a session that ended with one.
	StateFailed State = "failed"

	// StateAborted is an orchestration that was asked to stop before
	// reaching what it was for.
	StateAborted State = "aborted"

	// StateRefused is an orchestration the monitor would not accept.
	StateRefused State = "refused"
)

var (
	mu sync.RWMutex

	sessions       = make(map[string]*Session)
	orchestrations = make(map[string]*Orchestration)

	// participants is, per orchestration, the nodes whose instance monitor
	// carries its id. An orchestration is over when the last one drops it.
	participants = make(map[string]map[string]bool)

	// monitorOrchestration is the orchestration each instance monitor last
	// carried, so the one a node drops is known when its monitor stops
	// naming one.
	monitorOrchestration = make(map[string]string)

	// MaxEntries is how many ended entries of each kind are kept. A running
	// one is never dropped for the count: what is running is bounded by what
	// the node can run at once, and dropping it would lose the answer a
	// client is waiting for.
	MaxEntries = 1000

	// MaxAge is how long an ended entry is kept.
	MaxAge = time.Hour
)

// AddSession records an exec the daemon started.
func AddSession(s Session) {
	if s.ExecID == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if s.BeginAt.IsZero() {
		s.BeginAt = time.Now()
	}
	s.State = StateRunning
	sessions[s.ExecID] = &s
}

// EndSession records how an exec ended.
//
// It ends the exec and not the session: several execs share a session id, and
// ending by that id would have the second of them overwrite the first, which
// is how a command reaching two objects of a node reported one outcome.
//
// An exec the daemon never saw start is recorded all the same: the end
// carries what is needed to answer, and losing it because a message was
// missed would leave a client polling something that is over.
func EndSession(execID, sessionID string, state State, errS string, duration time.Duration) {
	if execID == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	s, ok := sessions[execID]
	if !ok {
		s = &Session{SessionID: sessionID, ExecID: execID, BeginAt: now.Add(-duration)}
		sessions[execID] = s
	}
	s.State = state
	s.Error = errS
	s.Duration = duration
	s.EndAt = &now
	purgeSessions()
}

// AddOrchestration records an orchestration the monitor accepted.
func AddOrchestration(o Orchestration) {
	if o.OrchestrationID == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if o.BeginAt.IsZero() {
		o.BeginAt = time.Now()
	}
	o.State = StateRunning
	orchestrations[o.OrchestrationID] = &o
}

// EndOrchestration records how an orchestration ended.
func EndOrchestration(id string, state State, errS string) {
	if id == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	o, ok := orchestrations[id]
	if !ok {
		o = &Orchestration{OrchestrationID: id, BeginAt: now}
		orchestrations[id] = o
	}
	o.State = state
	o.Error = errS
	o.EndAt = &now
	purgeOrchestrations()
}

// NoteMonitor records what the instance monitor of a node says about the
// orchestration of an object.
//
// Every node holds the monitor of every node, the daemon replicating them for
// the orchestration to converge at all, so this is what lets any node answer
// for an orchestration accepted by another. The alternative was for the node
// that accepted one to be named to the client, and for the client to be able
// to ask that node, which a client reaching the cluster through a floating
// address cannot: the address moves, and a switch is what moves it.
//
// id is empty when the monitor names no orchestration, which is how the end
// of one is seen: the id is unset on a node when the orchestration is reached
// there, so the last node to drop it ends it.
func NoteMonitor(path, node, id, globalExpect string, updatedAt time.Time) {
	mu.Lock()
	defer mu.Unlock()

	key := path + "/" + node
	previous := monitorOrchestration[key]
	if previous == id {
		return
	}
	if previous != "" {
		leave(previous, node)
	}
	if id == "" {
		delete(monitorOrchestration, key)
		return
	}
	monitorOrchestration[key] = id
	join(id, path, node, globalExpect, updatedAt)
}

// join adds a node to an orchestration, and records the orchestration when it
// is the first node to name it. The caller holds the lock.
func join(id, path, node, globalExpect string, updatedAt time.Time) {
	if participants[id] == nil {
		participants[id] = make(map[string]bool)
	}
	participants[id][node] = true

	o, ok := orchestrations[id]
	if !ok {
		beginAt := updatedAt
		if beginAt.IsZero() {
			beginAt = time.Now()
		}
		orchestrations[id] = &Orchestration{
			OrchestrationID: id,
			// Node is the node that accepted the orchestration, which only
			// the acceptance says. A monitor naming the id says the node is
			// in the orchestration, not that it accepted it, and every node
			// of the object names it.
			Path:         path,
			GlobalExpect: globalExpect,
			State:        StateRunning,
			BeginAt:      beginAt,
		}
		return
	}
	// The global expect of an orchestration is what it is for, and a node
	// joining late is as good a source for it as the first one.
	if o.GlobalExpect == "" {
		o.GlobalExpect = globalExpect
	}
	if o.Path == "" {
		o.Path = path
	}
}

// leave drops a node from an orchestration, and ends the orchestration when
// it was the last. The caller holds the lock.
func leave(id, node string) {
	if participants[id] != nil {
		delete(participants[id], node)
		if len(participants[id]) > 0 {
			return
		}
		delete(participants, id)
	}
	o, ok := orchestrations[id]
	if !ok {
		return
	}
	if o.EndAt != nil {
		// Ended already, and by something that knows how it ended: an
		// orchestration that was aborted or refused says so, where the
		// monitors falling silent only says it is over.
		return
	}
	now := time.Now()
	o.State = StateSucceeded
	o.EndAt = &now
	purgeOrchestrations()
}

// GetSessions returns the execs run under a session id, newest first.
//
// Several is the normal answer, not the exception: one command reaching two
// objects of a node runs two execs under one session id, each with its own
// outcome.
func GetSessions(sessionID string) []Session {
	mu.RLock()
	defer mu.RUnlock()
	l := make([]Session, 0, 1)
	for _, s := range sessions {
		if s.SessionID == sessionID {
			l = append(l, *s)
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].BeginAt.After(l[j].BeginAt) })
	return l
}

// GetOrchestration returns the orchestration of an id, and whether it is
// still known.
func GetOrchestration(id string) (Orchestration, bool) {
	mu.RLock()
	defer mu.RUnlock()
	o, ok := orchestrations[id]
	if !ok {
		return Orchestration{}, false
	}
	return *o, true
}

// Filter is what a listing is narrowed by. A zero value narrows nothing.
type Filter struct {
	States          []State
	OrchestrationID string
	ExecID          string
	Path            string
	Node            string
}

func (f Filter) match(s *Session) bool {
	if f.ExecID != "" && s.ExecID != f.ExecID {
		return false
	}
	if f.OrchestrationID != "" && s.OrchestrationID != f.OrchestrationID {
		return false
	}
	if f.Path != "" && s.Path != f.Path {
		return false
	}
	if f.Node != "" && s.Node != f.Node {
		return false
	}
	if len(f.States) == 0 {
		return true
	}
	for _, state := range f.States {
		if s.State == state {
			return true
		}
	}
	return false
}

// ListSessions returns the sessions a filter matches, newest first.
func ListSessions(f Filter) []Session {
	mu.RLock()
	defer mu.RUnlock()
	l := make([]Session, 0, len(sessions))
	for _, s := range sessions {
		if f.match(s) {
			l = append(l, *s)
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].BeginAt.After(l[j].BeginAt) })
	return l
}

// ListOrchestrations returns every orchestration still known, newest first.
func ListOrchestrations(f Filter) []Orchestration {
	mu.RLock()
	defer mu.RUnlock()
	l := make([]Orchestration, 0, len(orchestrations))
	for _, o := range orchestrations {
		if f.Path != "" && o.Path != f.Path {
			continue
		}
		if f.Node != "" && o.Node != f.Node {
			continue
		}
		if len(f.States) > 0 {
			var found bool
			for _, state := range f.States {
				if o.State == state {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		l = append(l, *o)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].BeginAt.After(l[j].BeginAt) })
	return l
}

// purgeSessions drops the ended sessions that are too old or too many.
// The caller holds the lock.
func purgeSessions() {
	deadline := time.Now().Add(-MaxAge)
	ended := make([]*Session, 0, len(sessions))
	for id, s := range sessions {
		if s.EndAt == nil {
			continue
		}
		if s.EndAt.Before(deadline) {
			delete(sessions, id)
			continue
		}
		ended = append(ended, s)
	}
	if len(ended) <= MaxEntries {
		return
	}
	sort.Slice(ended, func(i, j int) bool { return ended[i].EndAt.Before(*ended[j].EndAt) })
	for _, s := range ended[:len(ended)-MaxEntries] {
		delete(sessions, s.ExecID)
	}
}

// purgeOrchestrations drops the ended orchestrations that are too old or too
// many. The caller holds the lock.
func purgeOrchestrations() {
	deadline := time.Now().Add(-MaxAge)
	ended := make([]*Orchestration, 0, len(orchestrations))
	for id, o := range orchestrations {
		if o.EndAt == nil {
			continue
		}
		if o.EndAt.Before(deadline) {
			delete(orchestrations, id)
			forget(id)
			continue
		}
		ended = append(ended, o)
	}
	if len(ended) <= MaxEntries {
		return
	}
	sort.Slice(ended, func(i, j int) bool { return ended[i].EndAt.Before(*ended[j].EndAt) })
	for _, o := range ended[:len(ended)-MaxEntries] {
		delete(orchestrations, o.OrchestrationID)
		forget(o.OrchestrationID)
	}
}

// forget drops the bookkeeping of an orchestration that is no longer held.
// The caller holds the lock.
func forget(id string) {
	delete(participants, id)
	for key, orchestrationID := range monitorOrchestration {
		if orchestrationID == id {
			delete(monitorOrchestration, key)
		}
	}
}

// Purge drops what is too old, whether or not anything ended lately.
func Purge() {
	mu.Lock()
	defer mu.Unlock()
	purgeSessions()
	purgeOrchestrations()
}

// reset empties the tables. For tests.
func reset() {
	mu.Lock()
	defer mu.Unlock()
	sessions = make(map[string]*Session)
	orchestrations = make(map[string]*Orchestration)
	participants = make(map[string]map[string]bool)
	monitorOrchestration = make(map[string]string)
}

// IDString renders an id, empty when it carries none, so a session that
// belongs to no orchestration does not report belonging to the nil one.
func IDString(id xsession.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}
