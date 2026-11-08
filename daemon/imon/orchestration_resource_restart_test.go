package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

// newRestartTestManager returns the manager of a local instance wanted
// started, its avail local, and the instance of node2 avail peer.
func newRestartTestManager(local, peer status.T) *Manager {
	m := newTestManager(&pubSpy{})
	m.state.LocalExpect = instance.MonitorLocalExpectStarted
	m.nodeMonitor = map[string]node.Monitor{"node1": {State: node.MonitorStateIdle}}
	m.nodeStatus = map[string]node.Status{"node1": {}}
	m.instStatus["node1"] = instance.Status{Avail: local, Provisioned: provisioned.True, UpdatedAt: time.Now()}
	m.instStatus["node2"] = instance.Status{Avail: peer, Provisioned: provisioned.True, UpdatedAt: time.Now()}
	m.outdatedTimer = stoppedTimer()
	return m
}

// newRestartedTestManager returns the manager of a local instance whose
// monitored resource ip#0 was restarted, its last restart used, the exec
// ending at the returned time. The local instance status kept is the one
// that triggered the restart: ip#0 down, dated before the exec ended.
func newRestartedTestManager() (*Manager, time.Time) {
	m := newRestartTestManager(status.Down, status.Down)
	m.regularResourceOrchestrate.log = m.log
	m.standbyResourceOrchestrate.log = m.log
	m.regularResourceOrchestrate.scheduled = make(map[string]bool)
	m.standbyResourceOrchestrate.scheduled = make(map[string]bool)
	m.instConfig.ActorConfig = &instance.ActorConfig{
		Resources: instance.ResourceConfigs{"ip#0": {IsMonitored: true, Restart: 1}},
	}
	m.initialMonitorAction = instance.MonitorActionNone
	m.state.Resources = instance.ResourceMonitors{}
	m.state.Resources.Set("ip#0", instance.ResourceMonitor{Restart: &instance.ResourceMonitorRestart{Remaining: 0}})
	restartedAt := time.Now()
	m.regularResourceOrchestrate.restartedAt = restartedAt
	m.instStatus["node1"] = localStatusWithIP0(status.Down, restartedAt.Add(-3*time.Second))
	return m, restartedAt
}

// localStatusWithIP0 returns a local instance status of ip#0 st, dated at.
func localStatusWithIP0(st status.T, at time.Time) instance.Status {
	return instance.Status{
		Avail:       st,
		Provisioned: provisioned.True,
		UpdatedAt:   at,
		Resources:   instance.ResourceStatuses{"ip#0": resource.Status{Status: st}},
	}
}

// A restart is not judged on the status that triggered it. The status the
// start posts is read after the exec ended, maybe after an orchestration
// pass: that pass waits, the status it holds says ip#0 down with no restart
// left, and the monitor action would run though the restart succeeded.
func TestARestartIsNotJudgedOnTheStatusBeforeIt(t *testing.T) {
	m, _ := newRestartedTestManager()
	m.orchestrateResourceRestart()
	assert.True(t, m.state.MonitorActionExecutedAt.IsZero(), "no monitor action")
	assert.Equal(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect)
	assert.Equal(t, 0, m.state.Resources.Get("ip#0").Restart.Remaining, "the retry budget is untouched")
	assert.True(t, m.outdatedTimer.Stop(), "the outdated timer is armed to refresh the status, should none come")
}

// The status taken after the restart is the one judged: ip#0 up again, no
// monitor action runs, and its restarts are given back as before.
func TestARestartIsJudgedUpOnTheStatusAfterIt(t *testing.T) {
	m, restartedAt := newRestartedTestManager()
	m.orchestrateResourceRestart()
	m.instStatus["node1"] = localStatusWithIP0(status.Up, restartedAt.Add(time.Millisecond))
	m.orchestrateResourceRestart()
	assert.True(t, m.state.MonitorActionExecutedAt.IsZero(), "no monitor action")
	assert.Equal(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect)
	assert.Equal(t, 1, m.state.Resources.Get("ip#0").Restart.Remaining, "the restart count is reset to the config value")
}

// The status taken after the restart says ip#0 still down, with no restart
// left: the monitor action runs, as it did.
func TestARestartIsJudgedDownOnTheStatusAfterIt(t *testing.T) {
	m, restartedAt := newRestartedTestManager()
	m.orchestrateResourceRestart()
	m.instStatus["node1"] = localStatusWithIP0(status.Down, restartedAt.Add(time.Millisecond))
	m.orchestrateResourceRestart()
	assert.False(t, m.state.MonitorActionExecutedAt.IsZero(), "the monitor action runs")
	assert.Equal(t, instance.MonitorLocalExpectEvicted, m.state.LocalExpect)
}

// A standby restart holds the judgment of the resources the same way.
func TestAStandbyRestartIsNotJudgedOnTheStatusBeforeIt(t *testing.T) {
	m, restartedAt := newRestartedTestManager()
	m.regularResourceOrchestrate.restartedAt = time.Time{}
	m.standbyResourceOrchestrate.restartedAt = restartedAt
	m.orchestrateResourceRestart()
	assert.True(t, m.state.MonitorActionExecutedAt.IsZero(), "no monitor action")
}

// A failover instance down while a peer runs the object is no longer the one
// started: restarting its resources would run the object twice.
func TestAFailoverInstanceDownIsNotRestartedWhileAPeerRunsTheObject(t *testing.T) {
	m := newRestartTestManager(status.Down, status.Up)
	m.orchestrateResourceRestart()
	assert.Equal(t, instance.MonitorLocalExpectNone, m.state.LocalExpect)
}

// A regular restart armed before a peer took the object over is dropped with
// its timer: kept, the plan would skip it as already scheduled, and the timer
// would start the resources on this node too. A standby restart stays, as the
// standby resources run on every node.
func TestARestartArmedBeforeAPeerTookTheObjectIsDropped(t *testing.T) {
	m := newRestartTestManager(status.Down, status.Up)
	m.regularResourceOrchestrate.log = m.log
	m.standbyResourceOrchestrate.log = m.log
	fired := make(chan struct{}, 2)
	m.regularResourceOrchestrate.scheduled = map[string]bool{"app#1": true}
	m.regularResourceOrchestrate.scheduler = time.AfterFunc(50*time.Millisecond, func() { fired <- struct{}{} })
	m.standbyResourceOrchestrate.scheduled = map[string]bool{"disk#1": true}
	m.orchestrateResourceRestart()
	assert.Equal(t, instance.MonitorLocalExpectNone, m.state.LocalExpect)
	assert.Empty(t, m.regularResourceOrchestrate.scheduled, "the regular restart is dropped")
	assert.Nil(t, m.regularResourceOrchestrate.scheduler, "its timer is cancelled")
	assert.Equal(t, map[string]bool{"disk#1": true}, m.standbyResourceOrchestrate.scheduled, "the standby restart stays")
	select {
	case <-fired:
		t.Fatal("the timer of the dropped restart fired")
	case <-time.After(150 * time.Millisecond):
	}
}

// The instance running the object, or the one of an object down everywhere,
// stays the one started.
func TestAFailoverInstanceStaysStartedWhenNoPeerRunsTheObject(t *testing.T) {
	for _, c := range []struct{ local, peer status.T }{
		{status.Up, status.Down},
		{status.Down, status.Down},
		{status.Warn, status.Down},
	} {
		m := newRestartTestManager(c.local, c.peer)
		m.orchestrateResourceRestart()
		assert.Equalf(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect, "local %s, peer %s", c.local, c.peer)
	}
}

// A flex object runs on several nodes: a peer up says nothing of this one.
func TestAFlexInstanceStaysStartedWhileAPeerRuns(t *testing.T) {
	m := newRestartTestManager(status.Down, status.Up)
	m.objStatus.Topology = topology.Flex
	m.orchestrateResourceRestart()
	assert.Equal(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect)
}

// A resource stopped on purpose keeps the daemon from starting its instance
// on its own, as the instance flag does: the start would start the resource
// back. A peer takes the object over, and a start a user asks for still
// counts the instance.
func TestAnInstanceWithAResourceStoppedIsNoCandidateOfTheDaemon(t *testing.T) {
	m := newAffinityTestManager(instance.ActorConfig{})
	local := m.instStatus["node1"]
	local.Resources = instance.ResourceStatuses{"app#1": resource.Status{IsStopped: true}, "fs#1": resource.Status{}}
	m.instStatus["node1"] = local
	assert.False(t, m.isStartCandidateLeader(true), "node1 is no candidate of the daemon")
	assert.True(t, m.isStartCandidateLeader(false), "node1 is a candidate of a user start")
	assert.Equal(t, []string{"app#1"}, local.StoppedResources())
}

// A giveback is a user request: the instance to start is the leader of the
// placement, still flagged stopped or frozen, where the ha leader rule
// skipped it and handed the start to a peer that had lowered its own flag
// first, or that a user had unstopped.
func TestGivebackStartsTheLeaderStillFlaggedStopped(t *testing.T) {
	m := newAffinityTestManager(instance.ActorConfig{})
	local := m.instStatus["node1"]
	local.StoppedAt = time.Now()
	m.instStatus["node1"] = local
	assert.False(t, m.isStartCandidateLeader(true), "the daemon, by itself, skips node1")
	assert.True(t, m.isStartCandidateLeader(false), "a request starts it")

	local.StoppedAt = time.Time{}
	local.FrozenAt = time.Now()
	m.instStatus["node1"] = local
	assert.True(t, m.isStartCandidateLeader(false), "frozen, it takes part in a request too")
}

// No resource restarts while the instance is torn down or frozen, as v2: a
// restart scheduled before, as the one of the standby resources of a booting
// node, is dropped with its timer, not to start the resources in the middle
// of the unprovision.
func TestNoRestartWhileTheInstanceIsTornDown(t *testing.T) {
	for _, globalExpect := range []instance.MonitorGlobalExpect{
		instance.MonitorGlobalExpectPurged,
		instance.MonitorGlobalExpectUnprovisioned,
		instance.MonitorGlobalExpectDeleted,
		instance.MonitorGlobalExpectFrozen,
	} {
		m := newRestartTestManager(status.StandbyDown, status.Down)
		m.state.GlobalExpect = globalExpect
		m.standbyResourceOrchestrate.log = m.log
		m.regularResourceOrchestrate.log = m.log
		fired := make(chan struct{}, 1)
		m.standbyResourceOrchestrate.scheduled = map[string]bool{"disk#3": true}
		m.standbyResourceOrchestrate.scheduler = time.AfterFunc(50*time.Millisecond, func() { fired <- struct{}{} })
		m.orchestrateResourceRestart()
		assert.Empty(t, m.standbyResourceOrchestrate.scheduled, globalExpect)
		assert.Nil(t, m.standbyResourceOrchestrate.scheduler, globalExpect)
		select {
		case <-fired:
			t.Fatalf("%s: the timer of the dropped restart fired", globalExpect)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// A restart whose timer fires while the monitor runs another action, as the
// unprovision of a purge, is dropped: its start would replace the state of
// that action, whose end would be lost.
func TestARestartFiringDuringAnotherActionIsDropped(t *testing.T) {
	m := newRestartTestManager(status.StandbyDown, status.Down)
	m.standbyResourceOrchestrate.log = m.log
	for _, state := range []instance.MonitorState{instance.MonitorStateUnprovisionProgress, instance.MonitorStateWaitChildren} {
		m.state.State = state
		m.standbyResourceOrchestrate.scheduled = map[string]bool{"disk#3": true}
		m.resourceRestart([]string{"disk#3"}, true)
		assert.Equal(t, state, m.state.State, "the action or the wait goes on")
		assert.Empty(t, m.standbyResourceOrchestrate.scheduled, state)
	}
}

// A restart dropped before it ran gives back the retry its scheduling took:
// a resource down after a freeze keeps the restarts it had.
func TestADroppedRestartGivesBackItsRetry(t *testing.T) {
	m := newRestartTestManager(status.StandbyDown, status.Down)
	m.standbyResourceOrchestrate.log = m.log
	m.regularResourceOrchestrate.log = m.log
	m.state.Resources = instance.ResourceMonitors{}
	m.state.Resources.Set("disk#3", instance.ResourceMonitor{Restart: &instance.ResourceMonitorRestart{Remaining: 1}})
	m.standbyResourceOrchestrate.scheduled = map[string]bool{"disk#3": true}

	m.state.GlobalExpect = instance.MonitorGlobalExpectFrozen
	m.orchestrateResourceRestart()
	assert.Empty(t, m.standbyResourceOrchestrate.scheduled)
	assert.Equal(t, 2, m.state.Resources.Get("disk#3").Restart.Remaining, "the retry taken by the scheduling is given back")

	// A scheduling that took no retry, none being left, gives none back.
	m.state.Resources.Set("disk#3", instance.ResourceMonitor{Restart: &instance.ResourceMonitorRestart{Remaining: 0}})
	m.standbyResourceOrchestrate.scheduled = map[string]bool{"disk#3": false}
	m.orchestrateResourceRestart()
	assert.Equal(t, 0, m.state.Resources.Get("disk#3").Restart.Remaining)
}
