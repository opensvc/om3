package instance

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

func TestIsReplicationSource(t *testing.T) {
	up := ReferenceStatus{RID: "fs#1", Status: status.Up}
	down := ReferenceStatus{RID: "fs#1", Status: status.Down}
	warn := ReferenceStatus{RID: "ip#1", Status: status.Warn}
	optUp := ReferenceStatus{RID: "fs#2", Status: status.Up, Optional: true}
	stdbyUp := ReferenceStatus{RID: "disk#1", Status: status.StandbyUp}

	cases := []struct {
		name   string
		refs   []ReferenceStatus
		force  bool
		ok     bool
		forced bool
	}{
		{"active", []ReferenceStatus{up}, false, true, false},
		{"passive", []ReferenceStatus{down}, false, false, false},
		{"no reference resource", nil, false, false, false},
		{"no reference resource, forced", nil, true, false, false},
		{"only optional ones, up", []ReferenceStatus{optUp}, false, true, false},
		{"standby up with up", []ReferenceStatus{up, stdbyUp}, false, true, false},
		{"standby up only: a passive node", []ReferenceStatus{stdbyUp}, false, false, false},
		{"warn", []ReferenceStatus{up, warn}, false, false, false},
		{"warn, forced", []ReferenceStatus{up, warn}, true, true, true},
		{"down, forced", []ReferenceStatus{down}, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason, forced := IsReplicationSource(c.refs, c.force)
			require.Equal(t, c.ok, ok, reason)
			require.Equal(t, c.forced, forced)
			if !ok {
				require.NotEmpty(t, reason)
			}
		})
	}
}

func TestStatusReplicationSource(t *testing.T) {
	st := Status{Resources: ResourceStatuses{
		"fs#1":   {Status: status.Up, Type: "fs.flag"},
		"disk#1": {Status: status.Down, Type: "disk.drbd"},
		"app#1":  {Status: status.Down, Type: "app.simple"},
		"sync#1": {Status: status.Warn, Type: "sync.zfs"},
	}}
	ok, reason := st.ReplicationSource()
	require.True(t, ok, reason)

	st.Resources["fs#1"] = resource.Status{Status: status.Down, Type: "fs.flag"}
	ok, reason = st.ReplicationSource()
	require.False(t, ok)
	require.Equal(t, "reference resources down/down: fs#1:down", reason)

	delete(st.Resources, "fs#1")
	ok, _ = st.ReplicationSource()
	require.False(t, ok, "no reference resource left")
}
