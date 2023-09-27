package imon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/daemonhelper"
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
				{"obj", "status", "-r"},
				{"obj", "boot", "--local"},
			},
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
				{"obj", "status", "-r"},
				{"obj", "boot", "--local"},
			},
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
				{"obj", "status", "-r"},
			},
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
				{"obj", "status", "-r"},
			},
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
				{"obj", "status", "-r"},
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
				{"obj", "status", "-r"},
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
				{"obj", "status", "-r"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orchestrateTestfunc(t, c)
		})
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
				{"obj", "status", "-r"},
				{"obj", "boot", "--local"},
				{"obj", "start", "--local"},
			},
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
				{"obj", "status", "-r"},
				{"obj", "boot", "--local"},
				{"obj", "start", "--local"},
			},
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
				{"obj", "status", "-r"},
				{"obj", "start", "--local"},
			},
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
				{"obj", "status", "-r"},
				{"obj", "start", "--local"},
			},
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
				{"obj", "status", "-r"},
				{"obj", "start", "--local"},
			},
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
			expectedState:        instance.MonitorStateStartFailed,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,

			expectedCrm: [][]string{
				{"obj", "status", "-r"},
				{"obj", "start", "--local"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orchestrateTestfunc(t, c)
		})
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
				{"obj", "status", "-r"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orchestrateTestfunc(t, c)
		})
	}
}

func orchestrateTestfunc(t *testing.T, c tCase) {
	var err error
	maxRoutine := 10
	maxWaitTime := 2 * 1000 * time.Millisecond
	testCount.Add(1)
	now := time.Now()
	t.Logf("iteration %d starting", testCount.Load())

	setup := daemonhelper.Setup(t, nil)
	defer setup.Cancel()

	if c.bootID != "" {
		t.Logf("set node boot id for test to %s", c.bootID)
		bootid.Set(c.bootID)
	}

	istatD := istat.New()
	require.NoError(t, istatD.Start(setup.Ctx))

	//c := c
	p := path.T{Kind: path.KindSvc, Name: c.obj}

	if c.lastBootID != "" {
		t.Logf("set %s last instance boot id for test to %s", p, c.lastBootID)
		require.NoError(t, os.MkdirAll(filepath.Dir(lastBootIDFile(p)), 0755))
		require.NoError(t, updateLastBootID(p, c.lastBootID))
	}
	bus := pubsub.BusFromContext(setup.Ctx)

	for _, nmonState := range c.nodeMonitorStates {
		t.Logf("publish NodeMonitorUpdated state: %s", nmonState)
		nodeMonitor := node.Monitor{State: nmonState, StateUpdatedAt: time.Now(), GlobalExpectUpdatedAt: now, LocalExpectUpdatedAt: now}
		node.MonitorData.Set(hostname.Hostname(), nodeMonitor.DeepCopy())
		bus.Pub(&msgbus.NodeMonitorUpdated{Node: hostname.Hostname(), Value: nodeMonitor},
			pubsub.Label{"node", hostname.Hostname()})
	}

	t.Logf("publish initial node status with frozen %v", c.nodeFrozen)
	nodeStatus := node.Status{}
	if c.nodeFrozen {
		nodeStatus.FrozenAt = time.Now()
	}
	node.StatusData.Set(hostname.Hostname(), nodeStatus.DeepCopy())
	bus.Pub(&msgbus.NodeStatusUpdated{Node: hostname.Hostname(), Value: *nodeStatus.DeepCopy()},
		pubsub.Label{"node", hostname.Hostname()})

	initialReadyDuration := defaultReadyDuration
	defaultReadyDuration = 1 * time.Millisecond
	crm := crmBuilder(t, setup.Ctx, p, c.sideEffects)
	crmAction = crm.action
	defer func() {
		defaultReadyDuration = initialReadyDuration
		crmAction = nil
	}()

	factory := Factory{DrainDuration: setup.DrainDuration}
	evC := objectMonCreatorAndExpectationWatch(t, setup.Ctx, maxWaitTime, c, factory)

	cfgEtcFile := fmt.Sprintf("/etc/%s.conf", c.obj)
	setup.Env.InstallFile(c.srcFile, cfgEtcFile)
	t.Logf("--- starting icfg for %s", p)
	err = icfg.Start(setup.Ctx, p, filepath.Join(setup.Env.Root, cfgEtcFile), make(chan any, 20))
	require.Nil(t, err)

	t.Logf("waiting for watcher result")
	evImon := <-evC

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

	drainDuration := setup.DrainDuration + 15*time.Millisecond
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
	return append([][]string{}, c.calls...)
}

func crmBuilder(t *testing.T, ctx context.Context, p path.T, sideEffect map[string]sideEffect) *crm {
	bus := pubsub.BusFromContext(ctx)
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
		if len(cmdArgs) < 2 {
			err := fmt.Errorf("unexpected command %s", cmdArgs)
			t.Logf("--- crmAction error %s", err)
			return err
		}
		name := cmdArgs[0]
		action := cmdArgs[1]
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
			bus.Pub(&msgbus.InstanceStatusPost{Path: p, Node: hostname.Hostname(), Value: v},
				pubsub.Label{"path", p.String()},
				pubsub.Label{"node", hostname.Hostname()},
			)
			t.Logf("--- crmAction %s %v SetInstanceStatus %s avail:%s overall:%s provisioned:%s updated:%s frozen:%s", title, cmdArgs, p, v.Avail, v.Overall, v.Provisioned, v.UpdatedAt, v.FrozenAt)
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

// objectMonCreatorAndExpectationWatch returns a channel where we can read the InstanceMonitorUpdated for c
// that match c expectation, or latest received InstanceMonitorUpdated when duration is reached.
//
// It emulates discover omon creation for c (creates omon worker for c on first received InstanceConfigUpdated)
func objectMonCreatorAndExpectationWatch(t *testing.T, ctx context.Context, duration time.Duration, c tCase, factory Factory) <-chan *msgbus.InstanceMonitorUpdated {
	r := make(chan chan *msgbus.InstanceMonitorUpdated)

	go func() {
		var (
			evC = make(chan *msgbus.InstanceMonitorUpdated)

			p = path.T{Kind: path.KindSvc, Name: c.obj}

			monStarted bool

			latestInstanceMonitorUpdated *msgbus.InstanceMonitorUpdated
		)
		ctx, cancel := context.WithTimeout(ctx, duration)
		defer cancel()

		sub := pubsub.BusFromContext(ctx).Sub(t.Name() + ": discover & watcher")
		sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, pubsub.Label{"path", p.String()})
		sub.AddFilter(&msgbus.InstanceConfigUpdated{}, pubsub.Label{"path", p.String()})
		sub.Start()
		defer func() {
			_ = sub.Stop()
		}()

		t.Logf("watching InstanceMonitorUpdated and InstanceConfigUpdated for path: %s, max duration %s", p, duration)

		// serve response channel
		r <- evC

		for {
			select {
			case <-ctx.Done():
				evC <- latestInstanceMonitorUpdated
				return
			case i := <-sub.C:
				switch o := i.(type) {
				case *msgbus.InstanceConfigUpdated:
					if monStarted {
						continue
					}
					t.Logf("--- starting omon for %s", p)
					if err := omon.Start(ctx, p, o.Value, make(chan any, 100), factory); err != nil {
						t.Errorf("omon.Start failed: %s", err)
					}
					monStarted = true
				case *msgbus.InstanceMonitorUpdated:
					latestInstanceMonitorUpdated = o
					value := o.Value
					t.Logf("----  WATCH InstanceMonitorUpdated %s state: %s localExpect: %s globalExpect: %s isLeader: %v isHaLeader: %v",
						o.Path,
						value.State,
						value.LocalExpect,
						value.GlobalExpect,
						value.IsLeader,
						value.IsHALeader,
					)
					v := o.Value
					t.Logf("Verify if expected is reached for fast return")
					if c.expectedIsHALeader == v.IsHALeader &&
						c.expectedIsLeader == v.IsLeader &&
						c.expectedGlobalExpect == v.GlobalExpect &&
						c.expectedState == v.State &&
						c.expectedLocalExpect == v.LocalExpect {
						cancel()
					}
				}
			}
		}
	}()

	return <-r
}
