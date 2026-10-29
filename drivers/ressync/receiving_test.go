package ressync

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/hostname"
)

// A passive node judges the copy it receives only when the target makes it a
// destination: a local target copies on the source alone, and a node the
// target does not include receives nothing, which judged, never synced,
// breached its recovery point objective forever.
func TestReceivingNodenames(t *testing.T) {
	local := hostname.Hostname()
	var d T
	for _, tc := range []struct {
		name     string
		target   []string
		nodes    []string
		drpNodes []string
		want     []string
	}{
		{"nodes, a node", []string{"nodes"}, []string{"other", local}, nil, []string{local}},
		{"drpnodes, a drp node", []string{"drpnodes"}, []string{"other"}, []string{local}, []string{local}},
		{"local", []string{"local"}, []string{"other", local}, nil, nil},
		{"drpnodes, a node", []string{"drpnodes"}, []string{"other", local}, []string{"drp1"}, nil},
		{"nodes, a drp node", []string{"nodes"}, []string{"other"}, []string{local}, nil},
		{"nodes and drpnodes, a drp node", []string{"nodes", "drpnodes"}, []string{"other"}, []string{local}, []string{local}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, d.ReceivingNodenames(tc.target, tc.nodes, tc.drpNodes))
		})
	}
}
