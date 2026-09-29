package imon

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// A local status depending on the peers is refreshed when the peer
// counterpart of the resource changes status, or the peer instance changes
// availability, and not for a change nothing local depends on.
func TestPeerChangeSeenBy(t *testing.T) {
	dependent := instance.Status{Resources: instance.ResourceStatuses{
		"disk#1": {Status: status.Down, DependsOnPeers: true},
		"fs#1":   {Status: status.Down},
	}}
	independent := instance.Status{Resources: instance.ResourceStatuses{
		"fs#1": {Status: status.Down},
	}}
	peer := func(avail, disk, fs status.T) instance.Status {
		return instance.Status{Avail: avail, Resources: instance.ResourceStatuses{
			"disk#1": resource.Status{Status: disk},
			"fs#1":   resource.Status{Status: fs},
		}}
	}
	running := peer(status.Up, status.Up, status.Up)

	for _, tc := range []struct {
		name  string
		local instance.Status
		cur   instance.Status
		want  bool
	}{
		{"the peer stopped", dependent, peer(status.Down, status.Down, status.Down), true},
		{"the peer resource depended on changed", dependent, peer(status.Up, status.Warn, status.Up), true},
		{"another peer resource changed", dependent, peer(status.Warn, status.Up, status.Down), true},
		{"nothing changed", dependent, running, false},
		{"nothing local depends on the peers", independent, peer(status.Down, status.Down, status.Down), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := peerChangeSeenBy(tc.local, running, tc.cur)
			require.Equal(t, tc.want, got)
		})
	}
}
