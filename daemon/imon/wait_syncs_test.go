package imon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
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
