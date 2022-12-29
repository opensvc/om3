package daemondata

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/jsondelta"
)

func TestJournal(t *testing.T) {
	data := newData(nil).pending
	b, err := json.Marshal(data)
	require.NoError(t, err)

	t.Logf("b: %s", b)
	fooMon := &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectFrozen}
	fooCfg := &instance.Config{Checksum: "foo-cfg-sum"}
	barCfg := &instance.Config{Checksum: "bar-cfg-sum"}
	barStatus := &instance.Status{Avail: status.Up}

	cases := []jsondelta.Operation{
		{
			OpPath:  jsondelta.OperationPath{"cluster", "node", "node1", "instance", "foo"},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		},
		{
			OpPath:  jsondelta.OperationPath{"cluster", "node", "node1", "instance", "foo", "monitor"},
			OpValue: jsondelta.NewOptValue(fooMon),
			OpKind:  "replace",
		},
		{
			OpPath:  jsondelta.OperationPath{"cluster", "node", "node1", "instance", "foo", "config"},
			OpValue: jsondelta.NewOptValue(fooCfg),
			OpKind:  "replace",
		},
		{
			OpPath:  jsondelta.OperationPath{"cluster", "node", "node1", "instance", "bar"},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		},
		{
			OpPath:  jsondelta.OperationPath{"cluster", "node", "node1", "instance", "bar", "config"},
			OpValue: jsondelta.NewOptValue(barCfg),
			OpKind:  "replace",
		},
		{
			OpPath:  jsondelta.OperationPath{"cluster", "node", "node1", "instance", "bar", "status"},
			OpValue: jsondelta.NewOptValue(barStatus),
			OpKind:  "replace",
		},
	}

	for _, op := range cases {
		t.Run(op.OpPath.String()+" "+op.Kind(), func(t *testing.T) {
			patches := make(jsondelta.Patch, 0)
			patches = append(patches, op)
			b, err = patches.Apply(b)
			require.NoError(t, err)
		})
		require.False(t, t.Failed())
	}

	require.NoError(t, json.Unmarshal(b, &data))

	require.Equal(t, instance.MonitorGlobalExpectFrozen, data.Cluster.Node["node1"].Instance["foo"].Monitor.GlobalExpect)
	require.Equal(t, "foo-cfg-sum", data.Cluster.Node["node1"].Instance["foo"].Config.Checksum)
	require.Equal(t, "bar-cfg-sum", data.Cluster.Node["node1"].Instance["bar"].Config.Checksum)
	require.Equal(t, status.Up, data.Cluster.Node["node1"].Instance["bar"].Status.Avail)
}
