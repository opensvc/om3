package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASessionIsRunningUntilItsEndIsSeen(t *testing.T) {
	reset()
	AddSession(Session{ID: "s1", ExecID: "e-s1", Node: "n1", Origin: "api", Command: "om foo start"})

	s, ok := firstSession("s1")
	require.True(t, ok)
	assert.Equal(t, StateRunning, s.State)
	assert.Nil(t, s.EndAt)

	EndSession("e-s1", "s1", StateSucceeded, "", 3*time.Second)
	s, ok = firstSession("s1")
	require.True(t, ok)
	assert.Equal(t, StateSucceeded, s.State)
	assert.Equal(t, 3*time.Second, s.Duration)
	require.NotNil(t, s.EndAt)
}

// A failure carries what failed, which is the reason to ask at all.
func TestAFailedSessionKeepsItsError(t *testing.T) {
	reset()
	AddSession(Session{ID: "s1", ExecID: "e-s1"})
	EndSession("e-s1", "s1", StateFailed, "start failed: no such device", time.Second)

	s, _ := firstSession("s1")
	assert.Equal(t, StateFailed, s.State)
	assert.Equal(t, "start failed: no such device", s.Error)
}

// The end of a session whose start was missed is recorded all the same:
// dropping it would leave a client polling a session that is over.
func TestAnEndWithoutAStartIsRecorded(t *testing.T) {
	reset()
	EndSession("e-s1", "s1", StateSucceeded, "", time.Second)

	s, ok := firstSession("s1")
	require.True(t, ok)
	assert.Equal(t, StateSucceeded, s.State)
}

// Listing narrows on the state, and lists every state when asked for none:
// a client polling for the end of what it submitted is waiting for a session
// that has ended.
func TestListingNarrowsOnTheStateAndListsAllByDefault(t *testing.T) {
	reset()
	AddSession(Session{ID: "running", ExecID: "e-running"})
	AddSession(Session{ID: "ok", ExecID: "e-ok"})
	EndSession("e-ok", "ok", StateSucceeded, "", time.Second)
	AddSession(Session{ID: "ko", ExecID: "e-ko"})
	EndSession("e-ko", "ko", StateFailed, "boom", time.Second)

	assert.Len(t, ListSessions(Filter{}), 3, "every state when none is named")
	assert.Len(t, ListSessions(Filter{States: []State{StateRunning}}), 1)
	assert.Len(t, ListSessions(Filter{States: []State{StateFailed}}), 1)
	assert.Len(t, ListSessions(Filter{States: []State{StateSucceeded, StateFailed}}), 2)
}

// The sessions of one orchestration are found together, which is what a
// client that submitted an orchestrated action has an id for.
func TestTheSessionsOfAnOrchestrationAreFoundByItsID(t *testing.T) {
	reset()
	AddSession(Session{ID: "s1", ExecID: "e-s1", OrchestrationID: "o1"})
	AddSession(Session{ID: "s2", ExecID: "e-s2", OrchestrationID: "o1"})
	AddSession(Session{ID: "s3", ExecID: "e-s3", OrchestrationID: "o2"})
	AddSession(Session{ID: "s4", ExecID: "e-s4"})

	l := ListSessions(Filter{OrchestrationID: "o1"})
	assert.Len(t, l, 2)
	for _, s := range l {
		assert.Equal(t, "o1", s.OrchestrationID)
	}
}

// What ended is dropped once it is too old. What is running is not: it is
// bounded by what the node runs at once, and dropping it would lose the
// answer a client is waiting for.
func TestWhatEndedLongAgoIsDroppedAndWhatRunsIsKept(t *testing.T) {
	reset()
	AddSession(Session{ID: "old", ExecID: "e-old"})
	EndSession("e-old", "old", StateSucceeded, "", time.Second)
	AddSession(Session{ID: "running", ExecID: "e-running"})

	// Age it past the retention.
	mu.Lock()
	past := time.Now().Add(-2 * MaxAge)
	sessions["e-old"].EndAt = &past
	mu.Unlock()

	Purge()

	_, ok := firstSession("old")
	assert.False(t, ok, "an ended session past the retention is dropped")
	_, ok = firstSession("running")
	assert.True(t, ok, "a running session is kept whatever its age")
}

func TestOnlyTheNewestEndedSessionsAreKept(t *testing.T) {
	reset()
	saved := MaxEntries
	MaxEntries = 3
	defer func() { MaxEntries = saved }()

	for _, id := range []string{"1", "2", "3", "4", "5"} {
		AddSession(Session{ID: id, ExecID: "e-" + id})
		EndSession("e-"+id, id, StateSucceeded, "", time.Second)
	}

	assert.Len(t, ListSessions(Filter{}), 3)
	_, ok := firstSession("5")
	assert.True(t, ok, "the newest is kept")
	_, ok = firstSession("1")
	assert.False(t, ok, "the oldest is dropped")
}

func TestAnOrchestrationEndsAbortedOrRefused(t *testing.T) {
	reset()
	AddOrchestration(Orchestration{ID: "o1", Path: "svc1", GlobalExpect: "started"})
	o, ok := GetOrchestration("o1")
	require.True(t, ok)
	assert.Equal(t, StateRunning, o.State)

	EndOrchestration("o1", StateAborted, "")
	o, _ = GetOrchestration("o1")
	assert.Equal(t, StateAborted, o.State)

	AddOrchestration(Orchestration{ID: "o2"})
	EndOrchestration("o2", StateRefused, "node is frozen")
	o, _ = GetOrchestration("o2")
	assert.Equal(t, StateRefused, o.State)
	assert.Equal(t, "node is frozen", o.Error)
}

// An id nobody ever heard of and one that has been dropped are the same
// answer here. The api tells them apart from a running one, which is what a
// poller needs, by answering 410 rather than a state.
func TestAnUnknownIDIsReportedAsUnknown(t *testing.T) {
	reset()
	_, ok := firstSession("nope")
	assert.False(t, ok)
	_, ok = GetOrchestration("nope")
	assert.False(t, ok)
}

// firstSession is the one exec of a session id, for the tests that record one.
func firstSession(id string) (Session, bool) {
	l := GetSessions(id)
	if len(l) == 0 {
		return Session{}, false
	}
	return l[0], true
}

// One command reaching two objects of a node is two execs under one session
// id, each with its own object, outcome and duration. Keying the table by the
// session id had the second overwrite the first, so the node reported one of
// the two and hid the other.
func TestTwoExecsOfOneSessionAreBothKept(t *testing.T) {
	reset()
	AddSession(Session{ID: "s1", ExecID: "e1", Path: "pod3"})
	AddSession(Session{ID: "s1", ExecID: "e2", Path: "pod6"})
	EndSession("e1", "s1", StateSucceeded, "", time.Second)
	EndSession("e2", "s1", StateFailed, "boom", 2*time.Second)

	l := GetSessions("s1")
	require.Len(t, l, 2)

	byPath := make(map[string]Session, 2)
	for _, s := range l {
		byPath[s.Path] = s
	}
	assert.Equal(t, StateSucceeded, byPath["pod3"].State)
	assert.Equal(t, StateFailed, byPath["pod6"].State, "the outcome of one is not the outcome of the other")
	assert.Equal(t, "boom", byPath["pod6"].Error)
	assert.Equal(t, 2*time.Second, byPath["pod6"].Duration)
}
