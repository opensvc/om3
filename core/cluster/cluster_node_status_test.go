package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/status"
)

func Test_TNodeStatus_DeepCopy(t *testing.T) {
	t1 := time.Now()
	value := &NodeStatus{
		Agent: "a",
		API:   12,
		Arbitrators: map[string]ArbitratorStatus{
			"node1": {
				Name:   "foo",
				Status: status.Up,
			},
		},
		Compat: 9,
		Env:    "test",
		Frozen: t1,
		Gen: map[string]uint64{
			"node1": uint64(19),
			"node2": uint64(10),
		},
		MinAvailMemPct:  9,
		MinAvailSwapPct: 15,
		Speaker:         true,
		Labels: map[string]string{
			"node1": "abc",
			"node2": "efg",
		},
	}

	copyValue := value.DeepCopy()
	require.Equal(t, copyValue.API, uint64(12))

	copyValue.API = 13
	require.Equal(t, value.API, uint64(12))
	copyValue.Arbitrators["node1"] = ArbitratorStatus{
		Name:   "foo",
		Status: status.Warn,
	}
	copyValue.Arbitrators["node2"] = ArbitratorStatus{
		Name:   "bar",
		Status: status.Warn,
	}

	require.Equal(t, value.Arbitrators["node1"].Name, "foo")
	require.Equal(t, value.Arbitrators["node1"].Status, status.Up)
	_, hasNode2 := value.Arbitrators["node2"]
	require.False(t, hasNode2)
	require.Equal(t, copyValue.Arbitrators["node2"].Name, "bar")

	newFrozen := time.Now()
	value.Frozen = newFrozen
	require.True(t, value.Frozen.After(copyValue.Frozen))

	require.True(t, copyValue.Frozen.Equal(t1))
	copyValue.Frozen = time.Now()
	require.True(t, copyValue.Frozen.After(value.Frozen))
}
