package imon

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonhelper"
	"opensvc.com/opensvc/daemon/icfg"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/daemon/omon"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	sideEffect struct {
		iStatus *instance.Status
		err     error
	}

	crm struct {
		calls  [][]string
		action func(title string, cmdArgs ...string) error
	}
)

func TestMain(m *testing.M) {
	testhelper.Main(m, func(args []string) {})
}

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
			expectedGlobalExpect: instance.MonitorGlobalExpectUnset,
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
			expectedGlobalExpect: instance.MonitorGlobalExpectUnset,
			expectedLocalExpect:  instance.MonitorLocalExpectUnset,
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
			expectedGlobalExpect: instance.MonitorGlobalExpectUnset,
			expectedLocalExpect:  instance.MonitorLocalExpectUnset,
			expectedIsLeader:     false,
			expectedIsHALeader:   false,

			expectedCrm: [][]string{
				{"ha-err", "status", "-r"},
				{"ha-err", "start", "--local"},
			},
		},
	}
	t.Skip("bypass waiting fix")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c := c
			setup := daemonhelper.Setup(t, nil)
			defer setup.Cancel()
			defaultReadyDuration = time.Millisecond
			cfgEtcFile := fmt.Sprintf("/etc/%s.conf", c.name)
			setup.Env.InstallFile(c.srcFile, cfgEtcFile)
			p := path.T{Kind: kind.Svc, Name: c.name}

			evC := watchEv(t, setup.Ctx, p, 350*time.Millisecond)

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

			err = icfg.Start(setup.Ctx, p, filepath.Join(setup.Env.Root, cfgEtcFile), make(chan any, 20), Factory)
			require.Nil(t, err)

			time.Sleep(5 * time.Millisecond)

			evImon := <-evC
			t.Logf("crm calls: %v", crm.calls)
			require.Equalf(t, c.expectedState, evImon.Value.State,
				"expected state %s found %s", c.expectedState, evImon.Value.State)
			require.Equalf(t, c.expectedGlobalExpect, evImon.Value.GlobalExpect,
				"expected global expect %s found %s", c.expectedGlobalExpect, evImon.Value.GlobalExpect)
			require.Equalf(t, c.expectedLocalExpect, evImon.Value.LocalExpect,
				"expected local expect %s found %s", c.expectedLocalExpect, evImon.Value.LocalExpect)
			require.Equalf(t, c.expectedIsLeader, evImon.Value.IsLeader,
				"expected IsLeader %v found %v", c.expectedIsLeader, evImon.Value.IsLeader)
			require.Equalf(t, c.expectedIsHALeader, evImon.Value.IsHALeader,
				"expected IsHALeader %v found %v", c.expectedIsHALeader, evImon.Value.IsHALeader)
			require.Equalf(t, c.expectedCrm, crm.calls,
				"expected calls %v, found %v", c.expectedCrm, crm.calls)
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

func crmBuilder(t *testing.T, ctx context.Context, p path.T, sideEffect map[string]sideEffect) *crm {
	bus := pubsub.BusFromContext(ctx)
	c := crm{
		calls: make([][]string, 0),
	}
	c.action = func(title string, cmdArgs ...string) error {
		t.Logf("--- crmAction %s %s", title, cmdArgs)
		c.calls = append(c.calls, cmdArgs)
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
			istatus := msgbus.InstanceStatusUpdated{
				Path: p,
				Node: "node1",
				Value: instance.Status{
					Avail:       se.iStatus.Avail,
					Overall:     se.iStatus.Overall,
					Kind:        p.Kind,
					Provisioned: se.iStatus.Provisioned,
					Optional:    se.iStatus.Optional,
					Updated:     time.Now(),
				},
			}
			t.Logf("--- crmAction %s %v publish %#v", title, cmdArgs, istatus)
			bus.Pub(istatus, pubsub.Label{"path", p.String()}, pubsub.Label{"node", "node1"})
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
