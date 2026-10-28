package imon

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// A peer waiting for the leader to provision reads its own status to tell
// the provision done. A status computed from the configuration the peer had
// before the one with the resources, n/a for an instance with no resource
// yet, does not end the orchestration: the status of the current
// configuration does.
func TestAProvisionWaitsForTheStatusOfTheCurrentConfig(t *testing.T) {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.state.State = instance.MonitorStateWaitLeader
	m.state.GlobalExpect = instance.MonitorGlobalExpectProvisioned
	m.state.OrchestrationID = uuid.New()
	m.instConfig.Checksum = "current"
	m.objStatus.Avail = status.NotApplicable
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.NotApplicable,
		Provisioned: provisioned.NotApplicable,
		ConfigCsum:  "former",
	}
	assert.False(t, m.provisionedClearIfReached(), "a status of the former configuration decides nothing")
	assert.False(t, m.state.OrchestrationIsDone)

	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.NotApplicable,
		Provisioned: provisioned.NotApplicable,
		ConfigCsum:  "current",
	}
	assert.True(t, m.provisionedClearIfReached(), "the status of the current configuration does")
}

// A status saying no checksum, from an agent that does not, is taken as
// current.
func TestAStatusWithNoChecksumIsCurrent(t *testing.T) {
	m := newTestManager(&pubSpy{})
	m.instConfig.Checksum = "current"
	m.instStatus[m.localhost] = instance.Status{Avail: status.Down}
	assert.True(t, m.isLocalStatusCurrent())
}

// An unprovision waits for the status of the current configuration before it
// queues an action, as it does before it tells the state reached.
func TestAnUnprovisionWaitsForTheStatusOfTheCurrentConfig(t *testing.T) {
	m := newUnprovisionedTestManager(instance.MonitorStateIdle)
	m.instConfig.Checksum = "current"
	st := m.instStatus[m.localhost]
	st.ConfigCsum = "former"
	m.instStatus[m.localhost] = st

	m.UnprovisionedFromIdle()
	assert.Equal(t, instance.MonitorStateIdle, m.state.State, "no action is queued on a status of the former configuration")
}
