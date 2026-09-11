package imon

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type crmRecordingPublisher struct {
	messages []pubsub.Messager
}

func (p *crmRecordingPublisher) Pub(message pubsub.Messager, _ ...pubsub.Label) {
	p.messages = append(p.messages, message)
}

func (p *crmRecordingPublisher) execs() []*msgbus.Exec {
	l := make([]*msgbus.Exec, 0)
	for _, m := range p.messages {
		if e, ok := m.(*msgbus.Exec); ok {
			l = append(l, e)
		}
	}
	return l
}

// An orchestration is in flight on every node of the object while it runs,
// including the nodes it asks nothing of, and a restart can fire on an idle
// one. Reading the id off the state would tag the restart with an
// orchestration it had no part in.
func TestAResourceRestartIsAStepOfNoOrchestration(t *testing.T) {
	SetCmdPathForTest("/bin/true")
	publisher := &crmRecordingPublisher{}
	m := &Manager{
		localhost: "node1",
		path:      naming.Path{Name: "svc1", Kind: naming.KindSvc},
		publisher: publisher,
		state:     instance.Monitor{OrchestrationID: uuid.New()},
	}
	m.logBase = plog.NewDefaultLogger()
	m.logSetOrchestrationID(uuid.Nil)

	require.NoError(t, m.crmResourceStart([]string{"app#1"}))
	require.NoError(t, m.crmStatus())

	execs := publisher.execs()
	require.Len(t, execs, 2)
	assert.True(t, execs[0].OrchestrationID.IsZero(),
		"the restart names no orchestration, though one is running")
	assert.False(t, execs[1].OrchestrationID.IsZero(),
		"a step of the orchestration still names it")
}
