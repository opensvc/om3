package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/plog"
)

// reservableDriver is a driver reserving its devices.
type reservableDriver struct {
	Driver
}

func (t *reservableDriver) Log() *plog.Logger                                     { return plog.NewDefaultLogger() }
func (t *reservableDriver) StatusLog() StatusLogger                               { return NewStatusLog() }
func (t *reservableDriver) ReservableDevices(context.Context) device.L            { return nil }
func (t *reservableDriver) IsSCSIPersistentReservationPreemptAbortDisabled() bool { return false }
func (t *reservableDriver) IsSCSIPersistentReservationEnabled() bool              { return true }
func (t *reservableDriver) PersistentReservationKey() string                      { return "0x1" }

// The daemon starting a resource preempts a reservation another node holds,
// as a takeover fences the node gone. Its restart of the resource does not:
// that is the node the reservation was taken from trying to get it back.
func TestOnlyTheStartOfATakeoverPreemptsAForeignReservation(t *testing.T) {
	ctx := context.Background()
	r := &reservableDriver{}

	t.Setenv(env.ActionOriginVar, string(env.ActionOriginUser))
	hdl := newSCSIPersistentRerservationHandle(ctx, r)
	require.NotNil(t, hdl)
	assert.False(t, hdl.Force, "a user start without --force")

	t.Setenv(env.ActionOriginVar, string(env.ActionOriginDaemonMonitor))
	assert.True(t, newSCSIPersistentRerservationHandle(ctx, r).Force, "a start of the daemon")

	t.Setenv(env.ResourceRestartVar, "true")
	assert.False(t, newSCSIPersistentRerservationHandle(ctx, r).Force, "a restart of the daemon")
}
