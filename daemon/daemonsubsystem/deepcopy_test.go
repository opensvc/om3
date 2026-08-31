package daemonsubsystem

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeepCopyKeepsCollectionsNonNil pins a deliberate departure from
// copying faithfully. These methods are the daemon's publication path,
// the fields below carry no omitempty, and hbctrl builds Alerts by
// appending to a nil slice, so an alert-free stream is nil here. Copying
// that nil through would serve "alerts": null where an api client has
// always been served "alerts": [].
func TestDeepCopyKeepsCollectionsNonNil(t *testing.T) {
	d := Daemon{Heartbeat: Heartbeat{Streams: []HeartbeatStream{{Type: "unicast"}}}}
	c := d.DeepCopy()

	require.NotNil(t, c.Heartbeat.LastMessages)
	require.NotNil(t, c.Heartbeat.Streams)
	require.NotNil(t, c.Heartbeat.Streams[0].Alerts)
	require.NotNil(t, c.Heartbeat.Streams[0].Peers)
	require.NotNil(t, c.Dns.Nameservers)

	b, err := json.Marshal(c)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(b, &got))
	hb := got["heartbeat"].(map[string]any)
	assert.Equal(t, []any{}, hb["last_messages"])
	stream := hb["streams"].([]any)[0].(map[string]any)
	assert.Equal(t, []any{}, stream["alerts"])
	assert.Equal(t, map[string]any{}, stream["peers"])
	assert.Equal(t, []any{}, got["dns"].(map[string]any)["nameservers"])
}

// TestDeepCopyKeepsEveryField guards the failure mode these methods were
// written with: building the copy by listing the fields wanted, which
// silently stops copying every field added afterwards. Daemon.Nodename
// and Heartbeat.SecretVersion and UpdatedAt were dropped that way.
func TestDeepCopyKeepsEveryField(t *testing.T) {
	d := Daemon{Nodename: "node1", Pid: 42}
	d.Heartbeat.UpdatedAt = d.StartedAt.AddDate(1, 0, 0)
	d.Heartbeat.SecretVersion.Main = 7
	d.Heartbeat.SecretVersion.Alternate = 9

	c := d.DeepCopy()
	assert.Equal(t, "node1", c.Nodename)
	assert.Equal(t, 42, c.Pid)
	assert.Equal(t, uint64(7), c.Heartbeat.SecretVersion.Main)
	assert.Equal(t, uint64(9), c.Heartbeat.SecretVersion.Alternate)
	assert.Equal(t, d.Heartbeat.UpdatedAt, c.Heartbeat.UpdatedAt)
}

func TestDeepCopySharesNothing(t *testing.T) {
	d := Daemon{
		Dns:       Dns{Nameservers: []string{"a"}},
		Heartbeat: Heartbeat{Streams: []HeartbeatStream{{Alerts: []Alert{{Message: "m"}}, Peers: map[string]HeartbeatStreamPeerStatus{"p": {Desc: "d"}}}}},
	}
	c := d.DeepCopy()

	d.Dns.Nameservers[0] = "mutated"
	d.Heartbeat.Streams[0].Alerts[0].Message = "mutated"
	d.Heartbeat.Streams[0].Peers["p"] = HeartbeatStreamPeerStatus{Desc: "mutated"}

	assert.Equal(t, "a", c.Dns.Nameservers[0])
	assert.Equal(t, "m", c.Heartbeat.Streams[0].Alerts[0].Message)
	assert.Equal(t, "d", c.Heartbeat.Streams[0].Peers["p"].Desc)
}
