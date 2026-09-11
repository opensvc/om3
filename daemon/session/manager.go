package session

import (
	"context"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	// Manager fills the session and orchestration tables from what the bus
	// says, and drops from them what is too old.
	Manager struct {
		ctx       context.Context
		cancel    context.CancelFunc
		log       *plog.Logger
		localhost string
		subQS     pubsub.QueueSizer
		wg        sync.WaitGroup
	}
)

// purgeInterval is how often what is too old is dropped, for the tables that
// nothing ended lately in: a table is otherwise only purged when something
// ends, and a quiet node would hold its last sessions for as long as it
// stayed quiet.
var purgeInterval = time.Minute

// NewManager returns the manager of the session and orchestration tables.
func NewManager(subQS pubsub.QueueSizer) *Manager {
	return &Manager{
		log:       plog.NewDefaultLogger().Attr("pkg", "daemon/session").WithPrefix("daemon: session: "),
		localhost: hostname.Hostname(),
		subQS:     subQS,
	}
}

func (t *Manager) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	errC := make(chan error)
	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		errC <- nil
		t.loop()
	}(errC)
	return <-errC
}

func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) startSubscriptions() *pubsub.Subscription {
	sub := pubsub.SubFromContext(t.ctx, "daemon.session", t.subQS)
	label := pubsub.Label{"node", t.localhost}
	sub.AddFilter(&msgbus.Exec{}, label)
	sub.AddFilter(&msgbus.ExecSuccess{}, label)
	sub.AddFilter(&msgbus.ExecFailed{}, label)
	// The instance monitors of every node reach every node, so this is what
	// lets any node answer for an orchestration another one accepted. It is
	// subscribed without the node label for that reason: the monitor of a
	// peer is the point.
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{})
	sub.AddFilter(&msgbus.NodeMonitorUpdated{})
	sub.AddFilter(&msgbus.ObjectOrchestrationAccepted{}, label)
	sub.AddFilter(&msgbus.ObjectOrchestrationEnd{}, label)
	sub.AddFilter(&msgbus.ObjectOrchestrationRefused{}, label)
	sub.Start()
	return sub
}

func (t *Manager) loop() {
	sub := t.startSubscriptions()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			Purge()
		case i := <-sub.C:
			t.handle(i)
		}
	}
}

// handle records what one message says.
func (t *Manager) handle(i any) {
	{
		switch m := i.(type) {
		case *msgbus.InstanceMonitorUpdated:
			NoteMonitor(
				m.Path.String(),
				m.Node,
				IDString(xsession.NewOrchestrationID(m.Value.OrchestrationID)),
				m.Value.GlobalExpect.String(),
				m.Value.GlobalExpectUpdatedAt,
			)
		case *msgbus.NodeMonitorUpdated:
			// A node orchestration is the same thing of a node rather than of
			// an object, and its monitor reaches every node the same way. It
			// is recorded with no object, which is what says it is of the
			// node.
			NoteMonitor(
				"",
				m.Node,
				IDString(xsession.NewOrchestrationID(m.Value.OrchestrationID)),
				m.Value.GlobalExpect.String(),
				m.Value.GlobalExpectUpdatedAt,
			)
		case *msgbus.Exec:
			AddExec(Exec{
				SessionID:       IDString(m.SessionID),
				ExecID:          IDString(m.ExecID),
				OrchestrationID: IDString(m.OrchestrationID),
				Node:            m.Node,
				Path:            pathOf(m.Labels),
				Origin:          m.Origin,
				RID:             m.RID,
				Title:           m.Title,
				Command:         m.Command,
			})
		case *msgbus.ExecSuccess:
			EndExec(IDString(m.ExecID), IDString(m.SessionID), StateSucceeded, "", m.ExitCode, m.Duration)
		case *msgbus.ExecFailed:
			EndExec(IDString(m.ExecID), IDString(m.SessionID), StateFailed, m.ErrS, m.ExitCode, m.Duration)
		case *msgbus.ObjectOrchestrationAccepted:
			AddOrchestration(Orchestration{
				OrchestrationID: m.ID,
				Node:            m.Node,
				Path:            m.Path.String(),
				GlobalExpect:    m.GlobalExpect.String(),
			})
		case *msgbus.ObjectOrchestrationEnd:
			state := StateSucceeded
			if m.Aborted {
				state = StateAborted
			}
			EndOrchestration(m.ID, state, "")
		case *msgbus.ObjectOrchestrationRefused:
			AddOrchestration(Orchestration{
				OrchestrationID: m.ID,
				Node:            m.Node,
				Path:            m.Path.String(),
			})
			EndOrchestration(m.ID, StateRefused, m.Reason)
		}
	}
}

// pathOf returns the object an exec is of, which the message carries as a
// label rather than a field.
func pathOf(labels pubsub.Labels) string {
	return labels["path"]
}
