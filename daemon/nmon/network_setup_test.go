package nmon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/errcontext"
)

// stubNetworkSetup replaces the network setup by one recording the names it
// is asked for, and holding until released, its error the one released.
func stubNetworkSetup(t *testing.T) (calls chan []string, release chan error) {
	calls = make(chan []string, 4)
	release = make(chan error)
	prev := networkSetup
	networkSetup = func(_ *object.Node, names ...string) error {
		calls <- names
		return <-release
	}
	t.Cleanup(func() { networkSetup = prev })
	return calls, release
}

func newNetworkSetupTestManager(t *testing.T, state node.MonitorState) *Manager {
	testhelper.Setup(t)
	m := newTestManager(&recordingPublisher{})
	m.ctx = context.Background()
	m.state.State = state
	m.networkSetupC = make(chan networkSetupResult, 1)
	m.nodeMonitor = make(map[string]node.Monitor)
	return m
}

func nextNetworkSetup(t *testing.T, calls chan []string) []string {
	t.Helper()
	select {
	case names := <-calls:
		return names
	case <-time.After(5 * time.Second):
		t.Fatal("no network setup")
		return nil
	}
}

func applyNetworkSetupDone(t *testing.T, m *Manager) {
	t.Helper()
	select {
	case r := <-m.networkSetupC:
		m.onNetworkSetupDone(r)
	case <-time.After(5 * time.Second):
		t.Fatal("no network setup end")
	}
}

// The setups asked while one runs are merged into one, run when it ends:
// the setup of all the networks takes in the ones of named networks.
func TestTheNetworkSetupsAskedWhileOneRunsAreMerged(t *testing.T) {
	calls, release := stubNetworkSetup(t)
	m := newNetworkSetupTestManager(t, node.MonitorStateIdle)

	m.requestNetworkSetup([]string{"a"}, nil, uuid.Nil)
	assert.Equal(t, []string{"a"}, nextNetworkSetup(t, calls))
	m.requestNetworkSetup([]string{"b"}, nil, uuid.Nil)
	m.requestNetworkSetup([]string{"a"}, nil, uuid.Nil)
	require.Len(t, m.networkSetupQueue, 1)
	assert.Equal(t, []string{"b", "a"}, m.networkSetupQueue[0].names)
	m.requestNetworkSetup(nil, nil, uuid.Nil)
	assert.Empty(t, m.networkSetupQueue[0].names, "all the networks")
	m.requestNetworkSetup([]string{"c"}, nil, uuid.Nil)
	assert.Empty(t, m.networkSetupQueue[0].names, "all the networks take in a named one")

	release <- nil
	applyNetworkSetupDone(t, m)
	assert.Empty(t, nextNetworkSetup(t, calls), "the setup merged runs, of all the networks")
	assert.Empty(t, m.networkSetupQueue)
	release <- nil
	applyNetworkSetupDone(t, m)
	assert.False(t, m.networkSetupRunning)
}

// The node ending its rejoin goes idle, and its instances start, once the
// networks are set up: an ip.cni needs the cni configuration of its network.
func TestTheNodeGoesIdleOnceTheNetworksAreSetUp(t *testing.T) {
	calls, release := stubNetworkSetup(t)
	m := newNetworkSetupTestManager(t, node.MonitorStateRejoin)

	m.requestNetworkSetup(nil, nil, uuid.Nil)
	nextNetworkSetup(t, calls)
	m.rejoined()
	assert.Equal(t, node.MonitorStateRejoin, m.state.State, "held while the first setup runs")

	release <- errors.New("a network failing to set up")
	applyNetworkSetupDone(t, m)
	assert.Equal(t, node.MonitorStateIdle, m.state.State, "idle once it ended, failed or not")

	m.state.State = node.MonitorStateRejoin
	m.rejoined()
	assert.Equal(t, node.MonitorStateIdle, m.state.State, "at once once the networks were set up")
}

// A request waiting for a setup is answered its error once it ran, the
// request merged into the setup run next when one runs.
func TestARequestWaitingForANetworkSetupIsAnsweredItsEnd(t *testing.T) {
	calls, release := stubNetworkSetup(t)
	m := newNetworkSetupTestManager(t, node.MonitorStateIdle)

	m.requestNetworkSetup([]string{"a"}, nil, uuid.Nil)
	nextNetworkSetup(t, calls)
	waiter := errcontext.New(context.Background())
	m.requestNetworkSetup([]string{"b"}, waiter, uuid.Nil)

	release <- nil
	applyNetworkSetupDone(t, m)
	assert.Equal(t, []string{"b"}, nextNetworkSetup(t, calls), "the one asked runs after the one running")
	setupErr := errors.New("bridge failing")
	release <- setupErr
	applyNetworkSetupDone(t, m)
	assert.ErrorIs(t, waiter.Receive(), setupErr)
}

// A node rejoined, waiting for the networks to be set up to go idle, keeps
// its rejoin grace period timer stopped when its configuration is loaded
// again: rearmed, the timer would freeze the node once idle.
func TestTheRejoinTimerOfANodeRejoinedIsNotRearmed(t *testing.T) {
	calls, release := stubNetworkSetup(t)
	m := newNetworkSetupTestManager(t, node.MonitorStateRejoin)
	m.startedAt = time.Now()
	m.nodeConfig.RejoinGracePeriod = 10 * time.Millisecond
	m.rejoinTicker = time.NewTicker(time.Hour)
	m.rejoinTicker.Stop()

	m.requestNetworkSetup(nil, nil, uuid.Nil)
	nextNetworkSetup(t, calls)
	m.rejoined()
	m.checkRejoinTicker()
	select {
	case <-m.rejoinTicker.C:
		t.Fatal("the rejoin timer was rearmed")
	case <-time.After(100 * time.Millisecond):
	}
	release <- nil
	applyNetworkSetupDone(t, m)
	assert.Equal(t, node.MonitorStateIdle, m.state.State)
}

// A setup logs with one session id, the one of the client following its
// logs: the setups of two sessions asked while one runs are queued apart,
// and the requests of the daemon, with none, merge into either.
func TestTheNetworkSetupsOfTwoSessionsAreRunApart(t *testing.T) {
	calls, release := stubNetworkSetup(t)
	m := newNetworkSetupTestManager(t, node.MonitorStateIdle)
	s1, s2 := uuid.New(), uuid.New()

	m.requestNetworkSetup([]string{"a"}, nil, uuid.Nil)
	nextNetworkSetup(t, calls)
	m.requestNetworkSetup([]string{"b"}, nil, s1)
	m.requestNetworkSetup([]string{"c"}, nil, s2)
	m.requestNetworkSetup([]string{"d"}, nil, uuid.Nil)
	m.requestNetworkSetup([]string{"e"}, nil, s1)
	require.Len(t, m.networkSetupQueue, 2)
	assert.Equal(t, s1, m.networkSetupQueue[0].sessionID)
	assert.Equal(t, []string{"b", "d", "e"}, m.networkSetupQueue[0].names)
	assert.Equal(t, s2, m.networkSetupQueue[1].sessionID)
	assert.Equal(t, []string{"c"}, m.networkSetupQueue[1].names)

	release <- nil
	applyNetworkSetupDone(t, m)
	assert.Equal(t, []string{"b", "d", "e"}, nextNetworkSetup(t, calls))
	release <- nil
	applyNetworkSetupDone(t, m)
	assert.Equal(t, []string{"c"}, nextNetworkSetup(t, calls))
	release <- nil
	applyNetworkSetupDone(t, m)
	assert.False(t, m.networkSetupRunning)
}
