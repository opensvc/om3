package imon

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonhelper"
	"github.com/opensvc/om3/daemon/icfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/omon"
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
)

func Test_Orchestrate_HA(t *testing.T) {
	type tCase struct {
		name        string
		srcFile     string
		sideEffects map[string]sideEffect

		expectedState        instance.MonitorState
		expectedGlobalExpect instance.MonitorGlobalExpect
		expectedLocalExpect  instance.MonitorLocalExpect
		expectedIsLeader     bool
		expectedIsHALeader   bool

		expectedCrm [][]string
	}
	cases := []tCase{
		{
			name:    "ha",
			srcFile: "./testdata/ha.conf",
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
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectStarted,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"ha", "status", "-r"},
				{"ha", "start", "--local"},
			},
		},

		{
			name:    "ha-unprov",
			srcFile: "./testdata/ha.conf",
			sideEffects: map[string]sideEffect{
				"status": {
					iStatus: &instance.Status{Avail: status.Down, Overall: status.Down, Provisioned: provisioned.False},
					err:     nil,
				},
				"start": {
					iStatus: &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.False},
					err:     nil,
				},
			},
			expectedState:        instance.MonitorStateIdle,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     true,
			expectedIsHALeader:   true,
			expectedCrm: [][]string{
				{"ha-unprov", "status", "-r"},
			},
		},

		{
			name:    "ha-err",
			srcFile: "./testdata/ha.conf",
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
			expectedState:        instance.MonitorStateStartFailed,
			expectedGlobalExpect: instance.MonitorGlobalExpectNone,
			expectedLocalExpect:  instance.MonitorLocalExpectNone,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,

			expectedCrm: [][]string{
				{"ha-err", "status", "-r"},
				{"ha-err", "start", "--local"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c := c
			setup := daemonhelper.Setup(t, nil)
			defer setup.Cancel()
			defaultReadyDuration = time.Millisecond
			cfgEtcFile := fmt.Sprintf("/etc/%s.conf", c.name)
			setup.Env.InstallFile(c.srcFile, cfgEtcFile)
			p := path.T{Kind: kind.Svc, Name: c.name}

			evC := watchEv(t, setup.Ctx, p, 500*time.Millisecond)

			t.Logf("Set initial node monitor value")
			databus := daemondata.FromContext(setup.Ctx)
			now := time.Now()
			nodeMonitor := node.Monitor{State: node.MonitorStateIdle, StateUpdated: time.Now(), GlobalExpectUpdated: now, LocalExpectUpdated: now}
			err := databus.SetNodeMonitor(nodeMonitor)
			require.Nil(t, err)

			go createOmon(t, setup.Ctx)

			crm := crmBuilder(t, setup.Ctx, p, c.sideEffects)
			crmAction = crm.action
			defer func() {
				crmAction = nil
			}()

			factory := Factory{DrainDuration: setup.DrainDuration}
			err = icfg.Start(setup.Ctx, p, filepath.Join(setup.Env.Root, cfgEtcFile), make(chan any, 20), factory)
			require.Nil(t, err)

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
		})
	}
}

func createOmon(t *testing.T, ctx context.Context) {
	t.Logf("--- create omon from discovered ConfigUpdated")
	defer t.Logf("--- create omon from discovered ConfigUpdated [done]")
	monStarted := make(map[string]bool)
	bus := pubsub.BusFromContext(ctx)
	sub := bus.Sub("createOmon " + t.Name())
	sub.AddFilter(msgbus.ConfigUpdated{})
	sub.Start()
	defer func() {
		_ = sub.Stop()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case i := <-sub.C:
			switch o := i.(type) {
			case msgbus.ConfigUpdated:
				p := o.Path
				if monStarted[p.String()] {
					continue
				}
				t.Logf("--- starting omon for %s", p)
				err := omon.Start(ctx, p, o.Value, make(chan any, 100))
				require.Nil(t, err)
				monStarted[p.String()] = true
			}
		}
	}
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
	dBus := daemondata.FromContext(ctx)
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
			err := errors.Errorf("unexpected command %s", cmdArgs)
			t.Logf("--- crmAction error %s", err)
			return err
		}
		name := cmdArgs[0]
		action := cmdArgs[1]
		if name != p.Name {
			err := errors.Errorf("unexpected object %s vs %s", name, p.Name)
			t.Logf("--- crmAction error %s", err)
			return err
		}
		se, ok := sideEffect[action]
		if !ok {
			err := errors.Errorf("unexpected action %s: %s", action, cmdArgs)
			t.Logf("--- crmAction error %s", err)
			return err
		}

		if se.iStatus != nil {
			v := instance.Status{
				Avail:       se.iStatus.Avail,
				Overall:     se.iStatus.Overall,
				Kind:        p.Kind,
				Provisioned: se.iStatus.Provisioned,
				Optional:    se.iStatus.Optional,
				Updated:     time.Now(),
			}
			require.NoError(t, dBus.SetInstanceStatus(p, v))
			t.Logf("--- crmAction %s %v SetInstanceStatus %s avail:%s overall:%s provisioned:%s updated:%s", title, cmdArgs, p, v.Avail, v.Overall, v.Provisioned, v.Updated)
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

func watchEv(t *testing.T, parent context.Context, p path.T, timeout time.Duration) chan msgbus.InstanceMonitorUpdated {
	evC := make(chan msgbus.InstanceMonitorUpdated)

	go func() {
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()
		var lastEv msgbus.InstanceMonitorUpdated
		pub := pubsub.BusFromContext(ctx)
		sub := pub.Sub("watchEv " + t.Name())
		sub.AddFilter(msgbus.InstanceMonitorUpdated{}, pubsub.Label{"path", p.String()})
		sub.Start()
		defer func() {
			_ = sub.Stop()
		}()

		for {
			select {
			case <-ctx.Done():
				evC <- lastEv
				return
			case i := <-sub.C:
				switch o := i.(type) {
				case msgbus.InstanceMonitorUpdated:
					lastEv = o
					value := o.Value
					t.Logf("----  WATCH InstanceMonitorUpdated %s state: %s localExpect: %s globalExpect: %s isLeader: %v isHaLeader: %v",
						o.Path,
						value.State,
						value.LocalExpect,
						value.GlobalExpect,
						value.IsLeader,
						value.IsHALeader)
				}
			}
		}
	}()
	return evC
}
