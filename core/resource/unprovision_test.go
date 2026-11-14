package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/util/plog"
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

// provisionerDriver is a driver with a provisioner, stopped and unprovisioned
// by the leader method, counting both.
type provisionerDriver struct {
	Driver
	varDir                 string
	stopped, unprovisioned int
}

func (t *provisionerDriver) VarDir() string             { return t.varDir }
func (t *provisionerDriver) Log() *plog.Logger          { return plog.NewDefaultLogger() }
func (t *provisionerDriver) Stop(context.Context) error { t.stopped++; return nil }
func (t *provisionerDriver) UnprovisionAsLeader(context.Context) error {
	t.unprovisioned++
	return nil
}

// A resource recorded not provisioned is stopped by an unprovision, as in v2,
// not unprovisioned: the record says its provision made nothing to remove. A
// resource recorded provisioned, or with no record, is unprovisioned.
func TestAResourceRecordedNotProvisionedIsStoppedNotUnprovisioned(t *testing.T) {
	ctx := context.Background()

	r := &provisionerDriver{varDir: t.TempDir()}
	require.NoError(t, setProvisionedValue(false, r))
	require.NoError(t, unprovision(ctx, r, true))
	assert.Equal(t, 1, r.stopped, "stopped")
	assert.Equal(t, 0, r.unprovisioned, "not unprovisioned")

	r = &provisionerDriver{varDir: t.TempDir()}
	require.NoError(t, setProvisionedValue(true, r))
	require.NoError(t, unprovision(ctx, r, true))
	assert.Equal(t, 1, r.unprovisioned, "recorded provisioned: unprovisioned")

	r = &provisionerDriver{varDir: t.TempDir()}
	require.NoError(t, unprovision(ctx, r, true))
	assert.Equal(t, 1, r.unprovisioned, "no record: unprovisioned")
	v, err := getProvisionedValue(r)
	require.NoError(t, err)
	assert.False(t, v, "recorded not provisioned after")
}

// stoppableDriver is a driver with a start and a stop, and a var directory
// for its stopped flag.
type stoppableDriver struct {
	Driver
	varDir  string
	stopped int
}

func (t *stoppableDriver) VarDir() string              { return t.varDir }
func (t *stoppableDriver) Start(context.Context) error { return nil }
func (t *stoppableDriver) Stop(context.Context) error  { t.stopped++; return nil }
func (t *stoppableDriver) Log() *plog.Logger           { return nil }
func (t *stoppableDriver) IsStandby() bool             { return false }
func (t *stoppableDriver) IsShared() bool              { return false }

// An unprovision of selected resources flags them stopped, as a stop of them
// does, for the daemon not to start them again: a share unprovisioned with
// restart set was exported again at once (card 299). An unprovision of the
// instance flags nothing: the daemon disables its monitor.
func TestUnprovisionOfASelectedResourceFlagsItStopped(t *testing.T) {
	r := &stoppableDriver{varDir: t.TempDir()}
	require.NoError(t, unprovisionStop(context.Background(), r, true))
	assert.Equal(t, 1, r.stopped)
	stopped, err := IsStopped(r)
	require.NoError(t, err)
	assert.False(t, stopped, "no resource selected")

	ctx := actioncontext.WithRID(context.Background(), "share#1")
	require.NoError(t, unprovisionStop(ctx, r, true))
	stopped, err = IsStopped(r)
	require.NoError(t, err)
	assert.True(t, stopped, "the resource selected")
}
