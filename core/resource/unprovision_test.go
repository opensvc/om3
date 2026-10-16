package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leaderOnlyDriver is a driver that unprovisions with the leader method only,
// as the zfs ones do. It implements what the unprovision of a follower asks
// of a driver, the rest of Driver left to the nil interface it embeds.
type leaderOnlyDriver struct {
	Driver
	shared, standby bool
	unprovisioned   int
}

func (t *leaderOnlyDriver) IsShared() bool  { return t.shared }
func (t *leaderOnlyDriver) IsStandby() bool { return t.standby }
func (t *leaderOnlyDriver) UnprovisionAsLeader(context.Context) error {
	t.unprovisioned++
	return nil
}

// A resource not shared exists on every node, each having provisioned its
// own: a follower unprovisions it as the leader does. A shared one is the
// leader's alone to unprovision.
func TestUnprovisionAsFollower(t *testing.T) {
	ctx := context.Background()

	r := &leaderOnlyDriver{}
	require.NoError(t, unprovisionAsFollower(ctx, r))
	assert.Equal(t, 1, r.unprovisioned, "not shared, not standby: unprovisioned on the follower too")

	r = &leaderOnlyDriver{standby: true}
	require.NoError(t, unprovisionAsFollower(ctx, r))
	assert.Equal(t, 1, r.unprovisioned, "not shared, standby: unprovisioned on the follower")

	r = &leaderOnlyDriver{shared: true}
	require.NoError(t, unprovisionAsFollower(ctx, r))
	assert.Equal(t, 0, r.unprovisioned, "shared: the leader's alone")
}
