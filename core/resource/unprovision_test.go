package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/provisioned"
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

// statedDriver is a driver with nothing to provision whose resource has a
// provisioned state, when it says so.
type statedDriver struct {
	Driver
	stated bool
	state  provisioned.T
	varDir string
}

func (t *statedDriver) IsDisabled() bool          { return false }
func (t *statedDriver) HasProvisionedState() bool { return t.stated }
func (t *statedDriver) VarDir() string            { return t.varDir }
func (t *statedDriver) Provisioned(context.Context) (provisioned.T, error) {
	return t.state, nil
}

// A driver with nothing to provision has the provisioned state of a resource
// with a provisioner when it says so: read from the driver at first, then
// cached, and cleared by an unprovision, whatever the driver still finds,
// as an address its leader has yet to remove from the configuration. Not
// saying so, it is n/a.
func TestProvisionedState(t *testing.T) {
	ctx := context.Background()
	r := &statedDriver{stated: true, state: provisioned.True, varDir: t.TempDir()}
	v, err := Provisioned(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, provisioned.True, v, "read from the driver at first")

	require.NoError(t, setProvisionedValue(false, r))
	v, err = Provisioned(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, provisioned.False, v, "cleared, the driver still finding it")

	r.stated = false
	v, err = Provisioned(ctx, r)
	require.NoError(t, err)
	assert.Equal(t, provisioned.NotApplicable, v)
}
