package imon

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// A purge unprovisions a mixed instance, as one whose provision failed half
// way, before deleting it: what its provisioned resources took, as a drbd
// resource up on the disk of a volume, would otherwise outlive the object.
// Here the local instance leads the unprovision, so it waits for the peer
// still provisioned, rather than going straight to the delete.
func TestAPurgeUnprovisionsAMixedInstance(t *testing.T) {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.path = naming.Path{Kind: naming.KindVol, Name: "obj"}
	m.state.GlobalExpect = instance.MonitorGlobalExpectPurged
	m.state.OrchestrationID = uuid.New()
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Warn,
		Provisioned: provisioned.Mixed,
	}
	m.instStatus["node2"] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.True,
	}

	m.orchestratePurged()
	require.Equal(t, instance.MonitorStateWaitNonLeader, m.state.State,
		"the mixed instance must be unprovisioned, after its peers, not deleted as it is")
}
