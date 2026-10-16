package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// A consumer refreshes its status when the local instance of a vol it uses
// changes what its volume resource relays, as a start blocker appearing,
// and not for a status of the vol only evaluated again.
func TestAVolumeChangeRefreshesItsConsumer(t *testing.T) {
	vol := naming.Path{Kind: naming.KindVol, Name: "v1"}
	volStatus := instance.Status{
		Avail:   status.StandbyUp,
		Overall: status.StandbyUp,
		Resources: instance.ResourceStatuses{
			"disk#1": resource.Status{Status: status.StandbyUp},
		},
	}
	m := &Manager{
		localhost:         "n1",
		log:               plog.NewDefaultLogger(),
		instStatus:        map[string]instance.Status{"n1": {}},
		eventRefreshTimer: time.NewTimer(time.Hour),
		volumeSeen:        map[naming.Path]string{vol: volumeSignature(volStatus)},
	}
	m.eventRefreshTimer.Stop()

	volStatus.UpdatedAt = time.Now()
	m.onVolumeInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{Path: vol, Node: "n1", Value: volStatus})
	assert.False(t, m.eventRefreshScheduled, "the vol status was evaluated again, unchanged")

	volStatus.Resources = instance.ResourceStatuses{
		"disk#1": resource.Status{Status: status.StandbyUp, StartBlocker: &resource.StartBlocker{Reason: "disk Outdated", Resync: true}},
	}
	m.onVolumeInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{Path: vol, Node: "n2", Value: volStatus})
	assert.False(t, m.eventRefreshScheduled, "a peer instance of the vol is not relayed")

	m.onVolumeInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{Path: vol, Node: "n1", Value: volStatus})
	assert.True(t, m.eventRefreshScheduled, "the start blocker of the vol is relayed")
}
