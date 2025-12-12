package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/status"
)

func Test_TStatus_DeepCopy(t *testing.T) {
	t1 := time.Now()
	value := &Status{
		Agent: "a",
		API:   12,
		Arbitrators: map[string]ArbitratorStatus{
			"node1": {
				URL:    "foo",
				Status: status.Up,
			},
		},
		Compat:   9,
		FrozenAt: t1,
		Gen: Gen{
			"node1": uint64(19),
			"node2": uint64(10),
		},
		IsOverloaded: false,
		IsLeader:     true,
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
		URL:    "foo",
		Status: status.Warn,
	}
	copyValue.Arbitrators["node2"] = ArbitratorStatus{
		URL:    "bar",
		Status: status.Warn,
	}

	require.Equal(t, value.Arbitrators["node1"].URL, "foo")
	require.Equal(t, value.Arbitrators["node1"].Status, status.Up)
	_, hasNode2 := value.Arbitrators["node2"]
	require.False(t, hasNode2)
	require.Equal(t, copyValue.Arbitrators["node2"].URL, "bar")

	newFrozen := time.Now()
	value.FrozenAt = newFrozen
	require.True(t, value.FrozenAt.After(copyValue.FrozenAt))

	require.True(t, copyValue.FrozenAt.Equal(t1))
	copyValue.FrozenAt = time.Now()
	require.True(t, copyValue.FrozenAt.After(value.FrozenAt))
}
