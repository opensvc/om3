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
		id       string
		log      *plog.Logger
		matchers []string
	}
)

func EnableAudit(ctx context.Context, id string, log *plog.Logger, matchers ...string) {
	t := &T{
		id:       id,
		log:      log,
		matchers: matchers,
	}
	enabled := make(chan bool)
	go t.enableAudit(ctx, enabled)
	<-enabled
}

func AttachActiveAuditIfAny(ctx context.Context, log *plog.Logger, matchers ...string) {
	reg := daemonctx.AuditRegistry(ctx)
	if reg == nil {
		return
	}
	sess, ok := reg.Snapshot()
	if !ok {
		return
	}
	log.HandleAuditStart(sess.Q, sess.Subsystems, matchers...)
}

func (t *T) enableAudit(ctx context.Context, enabled chan<- bool) {
	t.attachActiveAuditIfAny(ctx)
	sub := pubsub.SubFromContext(ctx, t.id, pubsub.WithQueueSize(1024))
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	sub.Start()
	defer func() {
		_ = sub.Stop()
	}()
	enabled <- true
	for {
		select {
		case <-ctx.Done():
			return
		case i := <-sub.C:
			switch c := i.(type) {
			case *msgbus.AuditStart:
				t.log.HandleAuditStart(c.Q, c.Subsystems, t.matchers...)
			case *msgbus.AuditStop:
				t.log.HandleAuditStop(c.Q, c.Subsystems, t.matchers...)
			}
		}
	}
}

func (t *T) attachActiveAuditIfAny(ctx context.Context) {
	AttachActiveAuditIfAny(ctx, t.log, t.matchers...)
}
