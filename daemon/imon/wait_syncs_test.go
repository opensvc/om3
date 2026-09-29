package imon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
)

func TestRunningSyncs(t *testing.T) {
	m := &Manager{
		localhost: "n1",
		instStatus: map[string]instance.Status{
			"n1": {Running: resource.RunningInfoList{{RID: "task#1"}, {RID: "sync#2"}, {RID: "sync#1"}}},
		},
	}
	require.Equal(t, []string{"sync#2", "sync#1"}, m.runningSyncs(), "the syncs only: a task running does not hold a stop")
	m.instStatus["n1"] = instance.Status{}
	require.Empty(t, m.runningSyncs())
}

func TestStopArgsInterruptSyncs(t *testing.T) {
	m := &Manager{}
	m.path, _ = naming.ParsePath("test/svc/db")
	require.Equal(t, []string{"test/svc/db", "instance", "stop"}, m.stopArgs())
	m.state.GlobalExpectOptions = instance.MonitorGlobalExpectOptionsStopped{InterruptSyncs: true}
	require.Equal(t, []string{"test/svc/db", "instance", "stop", "--interrupt-syncs"}, m.stopArgs())
	m.state.GlobalExpectOptions = instance.MonitorGlobalExpectOptionsPlacedAt{InterruptSyncs: true}
	require.Equal(t, []string{"test/svc/db", "instance", "stop", "--move-to", "n2", "--interrupt-syncs"}, m.stopArgs("--move-to", "n2"))
	require.False(t, m.setWaitSyncs(), "no wait when the syncs are interrupted")
}
