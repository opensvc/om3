package hbaudit

import (
	"context"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		id  string
		log *plog.Logger
	}
)

func EnableAudit(ctx context.Context, id string, log *plog.Logger) {
	t := &T{
		id:  id,
		log: log,
	}
	enabled := make(chan bool)
	go t.EnableAudit(ctx, enabled)
	<-enabled
}

func (t *T) EnableAudit(ctx context.Context, enabled chan<- bool) {
	t.attachActiveAuditIfAny(ctx)
	sub := pubsub.SubFromContext(ctx, "hb:"+t.id, pubsub.WithQueueSize(1024))
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	sub.Start()
	enabled <- true
	for {
		select {
		case <-ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.AuditStart:
				t.log.HandleAuditStart(c.Q, c.Subsystems, "hb", "hb:"+t.id)
			case *msgbus.AuditStop:
				t.log.HandleAuditStop(c.Q, c.Subsystems, "hb", "hb:"+t.id)
			}
		}
	}
}

func (t *T) onEvent(ev any) {
	switch c := ev.(type) {
	case *msgbus.AuditStart:
		t.log.HandleAuditStart(c.Q, c.Subsystems, "hb", "hb:"+t.id)
	case *msgbus.AuditStop:
		t.log.HandleAuditStop(c.Q, c.Subsystems, "hb", "hb:"+t.id)
	}
}

func (t *T) attachActiveAuditIfAny(ctx context.Context) {
	reg := daemonctx.AuditRegistry(ctx)
	if reg == nil {
		return
	}
	sess, ok := reg.Snapshot()
	if !ok {
		return
	}
	t.log.HandleAuditStart(sess.Q, sess.Subsystems, "hb", "hb:"+t.id)
}
