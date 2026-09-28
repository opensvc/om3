package ressync

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/status"
)

func TestIsSource(t *testing.T) {
	up := refStatus{rid: "fs#1", status: status.Up}
	down := refStatus{rid: "fs#1", status: status.Down}
	warn := refStatus{rid: "ip#1", status: status.Warn}
	optUp := refStatus{rid: "fs#2", status: status.Up, optional: true}
	stdbyUp := refStatus{rid: "disk#1", status: status.StandbyUp}

	cases := []struct {
		name   string
		refs   []refStatus
		force  bool
		ok     bool
		forced bool
	}{
		{"active", []refStatus{up}, false, true, false},
		{"passive", []refStatus{down}, false, false, false},
		{"no reference resource", nil, false, false, false},
		{"no reference resource, forced", nil, true, false, false},
		{"only optional ones, up", []refStatus{optUp}, false, true, false},
		{"standby up with up", []refStatus{up, stdbyUp}, false, true, false},
		{"standby up only: a passive node", []refStatus{stdbyUp}, false, false, false},
		{"warn", []refStatus{up, warn}, false, false, false},
		{"warn, forced", []refStatus{up, warn}, true, true, true},
		{"down, forced", []refStatus{down}, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason, forced := isSource(c.refs, c.force)
			require.Equal(t, c.ok, ok, reason)
			require.Equal(t, c.forced, forced)
			if !ok {
				require.NotEmpty(t, reason)
			}
		})
	}
}
