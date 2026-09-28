package hbtype

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// A peer running a later version publishes values this agent has no name
// for. They decode to an explicit unknown, or to undef, instead of failing
// the whole message, which is how a peer used to be found dead.
func TestAMessageWithValuesOfALaterVersionDecodes(t *testing.T) {
	msg := Msg{Kind: "full", Nodename: "n2", NodeData: node.Node{
		Monitor: node.Monitor{State: node.MonitorStateIdle, GlobalExpect: node.MonitorGlobalExpectNone, LocalExpect: node.MonitorLocalExpectNone},
		Instance: map[string]instance.Instance{
			"svc1": {
				Monitor: &instance.Monitor{State: instance.MonitorStateIdle, GlobalExpect: instance.MonitorGlobalExpectNone, LocalExpect: instance.MonitorLocalExpectNone},
				Status:  &instance.Status{Avail: status.Up, Overall: status.Up, Provisioned: provisioned.True},
			},
		},
	}}
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	s := string(b)
	for _, r := range [][2]string{
		{`"state":"idle"`, `"state":"a later state"`},
		{`"global_expect":"none"`, `"global_expect":"a later expect"`},
		{`"local_expect":"none"`, `"local_expect":"a later expect"`},
		{`"avail":"up"`, `"avail":"a later status"`},
		{`"provisioned":"true"`, `"provisioned":"a later value"`},
	} {
		require.Contains(t, s, r[0])
		s = strings.ReplaceAll(s, r[0], r[1])
	}
	var out Msg
	require.NoError(t, json.Unmarshal([]byte(s), &out))
	assert.Equal(t, node.MonitorStateUnknown, out.NodeData.Monitor.State)
	assert.Equal(t, node.MonitorGlobalExpectUnknown, out.NodeData.Monitor.GlobalExpect)
	assert.Equal(t, node.MonitorLocalExpectUnknown, out.NodeData.Monitor.LocalExpect)
	inst := out.NodeData.Instance["svc1"]
	assert.Equal(t, instance.MonitorStateUnknown, inst.Monitor.State)
	assert.Equal(t, instance.MonitorGlobalExpectUnknown, inst.Monitor.GlobalExpect)
	assert.Equal(t, instance.MonitorLocalExpectUnknown, inst.Monitor.LocalExpect)
	assert.Equal(t, status.Undef, inst.Status.Avail)
	assert.Equal(t, provisioned.Undef, inst.Status.Provisioned)

	// What decoded marshals back, so this node can still publish and serve it.
	_, err = json.Marshal(out)
	require.NoError(t, err)

	var p placement.Policy
	require.NoError(t, json.Unmarshal([]byte(`"a later policy"`), &p))
	assert.Equal(t, placement.Invalid, p)
	var ps placement.State
	require.NoError(t, json.Unmarshal([]byte(`"a later state"`), &ps))
	assert.Equal(t, placement.Undef, ps)
}
