package imon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/daemontesthelper"
	"github.com/opensvc/om3/daemon/icfg"
	"github.com/opensvc/om3/daemon/istat"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/bootid"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	sideEffect struct {
		iStatus *instance.Status
		events  []pubsub.Messager
		err     error
	}

	crm struct {
		crmSpy
		action func(title string, cmdArgs ...string) error
	}

	crmSpy struct {
		sync.RWMutex
		calls [][]string
	}

	tCase struct {
		name        string
		obj         string
		srcFile     string
		bootID      string
		lastBootID  string
		sideEffects map[string]sideEffect

		nodeMonitorStates []node.MonitorState
		nodeFrozen        bool

		expectedState        instance.MonitorState
		expectedGlobalExpect instance.MonitorGlobalExpect
		expectedLocalExpect  instance.MonitorLocalExpect
		expectedIsLeader     bool
		expectedIsHALeader   bool

		// expectedDeleteSuccess is true if check delete orchestration with a
		// successfully crm delete
		expectedDeleteSuccess bool

		// expectedDeleteFailed is true if we want to check delete orchestration
		// with a failure during crm delete
		expectedDeleteFailed bool

		expectedCrm [][]string
	}
)

func TestMain(m *testing.M) {
	testhelper.Main(m, func(args []string) {})
}

var (
	testCount atomic.Uint64
)

func Test_Orchestrate_HA_that_dont_call_start(t *testing.T) {
	cases := []tCase{
		{
			name:       "if boot is required but boot fails then state is boot failed",
			srcFile:    "./testdata/orchestrate-ha.conf",
			obj:        "obj",
			bootID:     "bootID2",
			lastBootID: "bootID1",
			sideEffects: map[string]sideEffect{
				"boot": {
					iStatus: &instance.Status{Avail: status.Warn, Overall: status.Down, Provisioned: provisioned.True},
					err:     errors.New("boot fails for test"),
				},
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateBootFailed,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "boot"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:       "if boot is required but boot fails with side effect avail UP then instance is local expect started",
			srcFile:    "./testdata/orchestrate-ha.conf",
			obj:        "obj",
			bootID:     "bootID2",
			lastBootID: "bootID1",
			sideEffects: map[string]sideEffect{
				"boot": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
				"status": {
					iStatus: &instance.Status{Avail: status.Warn, Overall: status.Warn, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "boot"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:    "if instance avail is up then instance has local expect started",
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:    "if node is frozen then instance has local expect none and is not leader", // frozen node => no orchestration
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			nodeFrozen:           true,
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:    "if node state is rejoin and not frozen then instance has local expect none and is not leader", // rejoin => no orchestration
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateRejoin},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
			},
		},

		{
			name:    "if nmon has no state then instance has local expect none and is not leader", // nmon state undef => no orchestration
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
			},
		},

		{
			name:    "if instance is not provisioned then it is leader",
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.False},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     true,
			expectedIsHALeader:   false,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
			},
			expectedDeleteSuccess: true,
		},
	}
	for _, c := range cases {
		if c.expectedDeleteSuccess {
			t.Run(c.name+" with delete failed", func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})

			// always add extra run with failed delete when expectedDeleteSuccess is set
			c.expectedDeleteFailed = true
			c.expectedDeleteSuccess = false
			t.Run(c.name+" with delete failed", func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})
		} else {
			t.Run(c.name, func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})
		}
	}
}

func Test_Orchestrate_HA_that_calls_start(t *testing.T) {
	cases := []tCase{
		{
			name:       "if instance last boot is not current node boot then boot is required and we call instance boot to ensure start able instance",
			srcFile:    "./testdata/orchestrate-ha.conf",
			obj:        "obj",
			bootID:     "bootID2",
			lastBootID: "bootID1",
			sideEffects: map[string]sideEffect{
				"boot": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
				"status": {
					iStatus: &instance.Status{Avail: status.Warn, Overall: status.Warn, Provisioned: provisioned.True},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "boot"},
				{"obj", "instance", "start"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:       "if boot is required but boot fails with avail down then instance is started",
			srcFile:    "./testdata/orchestrate-ha.conf",
			obj:        "obj",
			bootID:     "bootID2",
			lastBootID: "bootID1",
			sideEffects: map[string]sideEffect{
				"boot": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     errors.New("boot fails for test"),
				},
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "boot"},
				{"obj", "instance", "start"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:    "if ha instance avail is down then instance is started and local expect is started",
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "start"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:       "if orchestrate ha is already booted then no boot action is required, but instance will be started",
			srcFile:    "./testdata/orchestrate-ha.conf",
			obj:        "obj",
			bootID:     "bootID1",
			lastBootID: "bootID1",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "start"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:       "if orchestrate ha is already booted, but boot id is empty then no boot action is required, but instance will be started",
			srcFile:    "./testdata/orchestrate-ha.conf",
			obj:        "obj",
			bootID:     "",
			lastBootID: "bootID1",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "start"},
			},
			expectedDeleteSuccess: true,
		},

		{
			name:    "if orchestrate ha instance start action has error then instance state is start failed with local expect none and leader false",
			srcFile: "./testdata/orchestrate-ha.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     errors.New("start failed"),
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateStartFailure,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,

			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
				{"obj", "instance", "start"},
			},
			expectedDeleteSuccess: true,
		},
	}
	for _, c := range cases {
		if c.expectedDeleteSuccess {
			t.Run(c.name+" with delete failed", func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})

			// always add extra run with failed delete when expectedDeleteSuccess is set
			c.expectedDeleteFailed = true
			c.expectedDeleteSuccess = false
			t.Run(c.name+" with delete failed", func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})
		} else {
			t.Run(c.name, func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})
		}
	}
}

func Test_Orchestrate_No(t *testing.T) {
	cases := []tCase{
		{
			name:    "if instance avail is down then instance is not started and local expect is none",
			srcFile: "./testdata/orchestrate-no.conf",
			obj:     "obj",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.True},
					err:     nil,
				},
			},
			nodeMonitorStates:    []node.MonitorState{node.MonitorStateIdle},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"obj", "instance", "status", "-r"},
			},
			expectedDeleteSuccess: true,
		},
	}
	for _, c := range cases {
		if c.expectedDeleteSuccess {
			t.Run(c.name+" with delete failed", func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})

			// always add extra run with failed delete when expectedDeleteSuccess is set
			c.expectedDeleteFailed = true
			c.expectedDeleteSuccess = false
			t.Run(c.name+" with delete failed", func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})
		} else {
			t.Run(c.name, func(t *testing.T) {
				orchestrateTestFunc(t, c)
			})
		}
	}
}

func orchestrateTestFunc(t *testing.T, c tCase) {
	var err error
	maxRoutine := 10
	maxWaitTime := 2 * 1000 * time.Millisecond
	testCount.Add(1)
	now := time.Now()
	t.Logf("iteration %d starting", testCount.Load())

	setup := daemontesthelper.Setup(t, nil)
	defer setup.Cancel()

	if c.bootID != "" {
		t.Logf("set node boot id for test to %s", c.bootID)
		bootid.Set(c.bootID)
	}

	istatD := istat.New(pubsub.WithQueueSize(100))
	require.NoError(t, istatD.Start(setup.Ctx))

	setup.InstallFile("./testdata/nodes_info.json", "var/nodes_info.json")

	p := naming.Path{Kind: naming.KindSvc, Name: c.obj}

	if c.lastBootID != "" {
		t.Logf("set %s last instance boot id for test to %s", p, c.lastBootID)
		require.NoError(t, os.MkdirAll(filepath.Dir(lastBootIDFile(p)), 0755))
		require.NoError(t, updateLastBootID(p, c.lastBootID))
	}
	pub := pubsub.PubFromContext(setup.Ctx)

	for _, nmonState := range c.nodeMonitorStates {
		t.Logf("publish NodeMonitorUpdated state: %s", nmonState)
		pubTime := time.Now()
		nodeMonitor := node.Monitor{State: nmonState, StateUpdatedAt: pubTime, UpdatedAt: pubTime, GlobalExpectUpdatedAt: now, LocalExpectUpdatedAt: now}
		node.MonitorData.Set(hostname.Hostname(), nodeMonitor.DeepCopy())
		pub.Pub(&msgbus.NodeMonitorUpdated{Node: hostname.Hostname(), Value: nodeMonitor},
			pubsub.Label{"node", hostname.Hostname()})
	}

	t.Logf("publish initial node status with frozen %v", c.nodeFrozen)
	nodeStatus := node.Status{}
	if c.nodeFrozen {
		nodeStatus.FrozenAt = time.Now()
	}
	node.StatusData.Set(hostname.Hostname(), nodeStatus.DeepCopy())
	pub.Pub(&msgbus.NodeStatusUpdated{Node: hostname.Hostname(), Value: *nodeStatus.DeepCopy()},
		pubsub.Label{"node", hostname.Hostname()})

	initialReadyDuration := defaultReadyDuration
	defaultReadyDuration = 1 * time.Millisecond

	if c.expectedDeleteSuccess {
		c.sideEffects["delete"] = sideEffect{
			events: []pubsub.Messager{&msgbus.InstanceConfigDeleted{Path: p, Node: hostname.Hostname()}},
			err:    nil,
		}
	} else if c.expectedDeleteFailed {
		c.sideEffects["delete"] = sideEffect{
			err: fmt.Errorf("crm delete action failed"),
		}
	}
	crm := crmBuilder(t, setup, p, c.sideEffects)
	testCRMAction = crm.action
	defer func() {
		defaultReadyDuration = initialReadyDuration
		testCRMAction = nil
	}()

	evC, errC := waitExpectations(t, setup, maxWaitTime, c)

	factory := Factory{
		DrainDuration: setup.DrainDuration,
		DelayDuration: 50 * time.Millisecond,
		SubQS:         pubsub.WithQueueSize(100),
	}
	objectMonCreator(t, setup, c, factory)

	cfgEtcFile := fmt.Sprintf("/etc/%s.conf", c.obj)
	setup.Env.InstallFile(c.srcFile, cfgEtcFile)
	t.Logf("--- starting icfg for %s", p)
	err = icfg.Start(setup.Ctx, p, filepath.Join(setup.Env.Root, cfgEtcFile), make(chan any, 20))
	require.Nil(t, err)

	t.Logf("waiting for watcher result")
	evImon, err := <-evC, <-errC
	assert.NoError(t, err)

	calls := crm.getCalls()
	t.Logf("crm calls: %v", calls)

	t.Logf("verify state")
	assert.Equalf(t, c.expectedState.String(), evImon.Value.State.String(),
		"expected state %s found %s", c.expectedState, evImon.Value.State)

	t.Logf("verify global expect")
	assert.Equalf(t, c.expectedGlobalExpect.String(), evImon.Value.GlobalExpect.String(),
		"expected global expect %s found %s", c.expectedGlobalExpect, evImon.Value.GlobalExpect)

	t.Logf("verify local expect")
	assert.Equalf(t, c.expectedLocalExpect.String(), evImon.Value.LocalExpect.String(),
		"expected local expect %s found %s", c.expectedLocalExpect, evImon.Value.LocalExpect)

	t.Logf("verify leader")
	assert.Equalf(t, c.expectedIsLeader, evImon.Value.IsLeader,
		"expected IsLeader %v found %v", c.expectedIsLeader, evImon.Value.IsLeader)

	t.Logf("verify ha leader")
	assert.Equalf(t, c.expectedIsHALeader, evImon.Value.IsHALeader,
		"expected IsHALeader %v found %v", c.expectedIsHALeader, evImon.Value.IsHALeader)

	t.Logf("verify calls")
	assert.Equalf(t, c.expectedCrm, calls,
		"expected calls %v, found %v", c.expectedCrm, calls)

	var deleteTest string
	require.False(t, c.expectedDeleteFailed && c.expectedDeleteSuccess,
		"can't test with both expectedDeleteFailed and expectedDeleteSuccess")
	switch {
	case c.expectedDeleteFailed:
		deleteTest = "try delete failed orchestration"
	case c.expectedDeleteSuccess:
		deleteTest = "try delete success orchestration "
	}
	if deleteTest != "" {
		t.Run(deleteTest, func(t *testing.T) {
			stateC, errC := waitNmonStates(setup.Ctx, "waiting for delete, deleting", 2*time.Second, p,
				instance.MonitorStateDeleteSuccess,
				instance.MonitorStateDeleteProgress)
			g := instance.MonitorGlobalExpectDeleted
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			value := instance.MonitorUpdate{
				GlobalExpect:             &g,
				CandidateOrchestrationID: uuid.New(),
			}
			msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, hostname.Hostname(), value)
			t.Logf("try delete orchestration with : %v", msg)
			pub.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, pubsub.Label{"origin", "api"})
			require.NoError(t, setImonErr.Receive())

			t.Logf("waiting for delete, deleting")
			state, err := <-stateC, <-errC
			require.NoError(t, err)
			t.Logf("found expected state: %s", state)

			// deleting state is published before crm call is done, delay getCalls
			time.Sleep(50 * time.Millisecond)
			calls = crm.getCalls()
			t.Logf("crm calls: %v", calls)
			t.Logf("verify last call is a delete call")
			expectedCalls := [][]string{[]string{"obj", "instance", "delete"}}
			assert.Equalf(t, expectedCalls, calls,
				"expected calls %v, found %v", expectedCalls, calls)
		})
	}

	drainDuration := setup.DrainDuration + 30*time.Millisecond

	t.Logf("setup cancel and wait for drain duration %s", drainDuration)
	setup.Cancel()
	time.Sleep(drainDuration)

	t.Logf("Verify goroutine counts")
	numGoroutine := runtime.NumGoroutine()
	t.Logf("iteration %d goroutines: %d", testCount.Load(), numGoroutine)
	if numGoroutine > maxRoutine {
		buf := make([]byte, 1<<16)
		runtime.Stack(buf, true)
		require.LessOrEqualf(t, numGoroutine, maxRoutine, "end test %d goroutines:\n %s", testCount.Load(), buf)
	}

	t.Logf("iteration %d duration: %s now: %s", testCount.Load(), time.Now().Sub(now), time.Now())
}

func (c *crmSpy) addCall(cmdArgs ...string) {
	c.Lock()
	defer c.Unlock()
	c.calls = append(c.calls, cmdArgs)
}

func (c *crmSpy) getCalls() [][]string {
	c.RLock()
	defer c.RUnlock()
	calls := append([][]string{}, c.calls...)
	c.calls = make([][]string, 0)
	return calls
}

func crmBuilder(t *testing.T, setup *daemontesthelper.D, p naming.Path, sideEffect map[string]sideEffect) *crm {
	ctx := setup.Ctx
	pub := pubsub.PubFromContext(ctx)
	c := crm{
		crmSpy: crmSpy{
			RWMutex: sync.RWMutex{},
			calls:   make([][]string, 0),
		},
		action: nil,
	}
	c.action = func(title string, cmdArgs ...string) error {
		t.Logf("--- crmAction %s %s", title, cmdArgs)
		c.addCall(cmdArgs...)
		if len(cmdArgs) < 3 {
			err := fmt.Errorf("unexpected command %s", cmdArgs)
			t.Logf("--- crmAction error %s", err)
			return err
		}
		name := cmdArgs[0]
		action := cmdArgs[2]
		if name != p.Name {
			err := fmt.Errorf("unexpected object %s vs %s", name, p.Name)
			t.Logf("--- crmAction error %s", err)
			return err
		}
		se, ok := sideEffect[action]
		if !ok {
			err := fmt.Errorf("unexpected action %s: %s", action, cmdArgs)
			t.Logf("--- crmAction error %s", err)
			return err
		}

		if se.iStatus != nil {
			v := instance.Status{
				Avail:       se.iStatus.Avail,
				Overall:     se.iStatus.Overall,
				Provisioned: se.iStatus.Provisioned,
				Optional:    se.iStatus.Optional,
				UpdatedAt:   time.Now(),
				FrozenAt:    time.Time{},
			}
			pub.Pub(&msgbus.InstanceStatusPost{Path: p, Node: hostname.Hostname(), Value: v},
				pubsub.Label{"namespace", p.Namespace},
				pubsub.Label{"path", p.String()},
				pubsub.Label{"node", hostname.Hostname()},
			)
			t.Logf("--- crmAction %s %v SetInstanceStatus %s avail:%s overall:%s provisioned:%s updated:%s frozen:%s", title, cmdArgs, p, v.Avail, v.Overall, v.Provisioned, v.UpdatedAt, v.FrozenAt)
		}

		for _, e := range se.events {
			t.Logf("--- crmAction %s %v publish sid effect %s %v", title, cmdArgs, reflect.TypeOf(e), e)
			pub.Pub(e,
				pubsub.Label{"namespace", p.Namespace},
				pubsub.Label{"path", p.String()},
				pubsub.Label{"node", hostname.Hostname()},
			)
		}
		if se.err != nil {
			t.Logf("--- crmAction %s %v error %s", title, cmdArgs, se.err)
		} else {
			t.Logf("--- crmAction %s %v done", title, cmdArgs)
		}
		return se.err
	}
	return &c
}

// objectMonCreator emulates discover omon creation for c (creates omon worker for c on first received InstanceConfigUpdated)
func objectMonCreator(t *testing.T, setup *daemontesthelper.D, c tCase, factory Factory) {
	var (
		p = naming.Path{Kind: naming.KindSvc, Name: c.obj}

		monStarted bool
	)

	ctx := setup.Ctx
	sub := pubsub.SubFromContext(ctx, t.Name()+": discover")
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, pubsub.Label{"path", p.String()})
	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, pubsub.Label{"path", p.String()})
	sub.Start()

	go func() {
		defer func() {
			_ = sub.Stop()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case i := <-sub.C:
				switch o := i.(type) {
				case *msgbus.InstanceConfigDeleted:
					monStarted = false
					time.Sleep(10 * time.Millisecond)
				case *msgbus.InstanceConfigUpdated:
					if !monStarted {
						t.Logf("--- starting omon for %s", p)
						if err := omon.Start(ctx, pubsub.WithQueueSize(50), p, o.Value, factory); err != nil {
							t.Errorf("omon.Start failed: %s", err)
						}
						monStarted = true
					}
				}
			}
		}
	}()
}

// waitExpectations watches for InstanceMonitorUpdated until matched expectation from c or reached timeout
// when timeout is reached, error is non nil and event is the latest event published or zero value
func waitExpectations(t *testing.T, setup *daemontesthelper.D, timeout time.Duration, c tCase) (<-chan *msgbus.InstanceMonitorUpdated, <-chan error) {
	parent := setup.Ctx
	evC := make(chan *msgbus.InstanceMonitorUpdated)
	errC := make(chan error)
	p := naming.Path{Kind: naming.KindSvc, Name: c.obj}
	latestInstanceMonitorUpdated := &msgbus.InstanceMonitorUpdated{}

	ctx, cancel := context.WithTimeout(parent, timeout)

	sub := pubsub.SubFromContext(ctx, t.Name()+": wait expectations")
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, pubsub.Label{"path", p.String()})
	sub.Start()

	go func() {
		t.Logf("watching InstanceMonitorUpdated for path: %s, max duration %s", p, timeout)
		defer cancel()
		defer func() {
			_ = sub.Stop()
		}()

		for {
			select {
			case <-ctx.Done():
				evC <- latestInstanceMonitorUpdated
				errC <- ctx.Err()
				return
			case i := <-sub.C:
				switch o := i.(type) {
				case *msgbus.InstanceMonitorUpdated:
					latestInstanceMonitorUpdated = o
					value := o.Value

					v := o.Value
					if c.expectedIsHALeader == v.IsHALeader &&
						c.expectedIsLeader == v.IsLeader &&
						c.expectedGlobalExpect == v.GlobalExpect &&
						c.expectedState == v.State &&
						c.expectedLocalExpect == v.LocalExpect {
						t.Logf("----  matched InstanceMonitorUpdated %s state: %s localExpect: %s globalExpect: %s isLeader: %v isHaLeader: %v",
							o.Path,
							value.State,
							value.LocalExpect,
							value.GlobalExpect,
							value.IsLeader,
							value.IsHALeader,
						)
						evC <- latestInstanceMonitorUpdated
						errC <- nil
						return
					}
				}
			}
		}
	}()

	return evC, errC
}

func waitNmonStates(ctx context.Context, desc string, d time.Duration, p naming.Path, states ...instance.MonitorState) (<-chan instance.MonitorState, <-chan error) {
	stateC := make(chan instance.MonitorState)
	errC := make(chan error)

	go func() {
		sub := pubsub.SubFromContext(ctx, desc)
		sub.AddFilter(&msgbus.InstanceMonitorUpdated{},
			[]pubsub.Label{{"path", p.String()}, {"node", hostname.Hostname()}}...)
		sub.Start()
		defer func() {
			go func() {
				_ = sub.Stop()
			}()
		}()

		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				stateC <- instance.MonitorStateInit
				errC <- ctx.Err()
				return
			case i := <-sub.C:
				if msg, ok := i.(*msgbus.InstanceMonitorUpdated); ok {
					if msg.Value.State.IsOneOf(states...) {
						stateC <- msg.Value.State
						errC <- ctx.Err()
						return
					}
				}
			}
		}
	}()
	return stateC, errC
}
