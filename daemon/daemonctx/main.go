package daemonctx

import (
	"context"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/subdaemon"
)

type (
	contextKey string
)

var (
	contextDaemon         = contextKey("daemon")
	contextHBSendQueue    = contextKey("hb-sendQ")
	contextHBRecvMsgQueue = contextKey("hb-recv-msg-queue")
	contextUuid           = contextKey("uuid")
	contextListenAddr     = contextKey("listen-addr")
)

func (c contextKey) String() string {
	return "daemonctx." + string(c)
}

// HBRecvMsgQ function returns HBRecvMsgQ from context
// hb component send the hbrx decoded message from peers on this queue
func HBRecvMsgQ(ctx context.Context) (hbRecvQ chan<- *hbtype.Msg) {
	var ok bool
	hbRecvQ, ok = ctx.Value(contextHBRecvMsgQueue).(chan<- *hbtype.Msg)
	if ok {
		return
	}
	return nil
}

// WithHBRecvMsgQ function returns copy of parent with HBRecvMsgQ
// the queue used by daemondata to retrieve hb rx decoded messages
func WithHBRecvMsgQ(parent context.Context, hbRecvQ chan<- *hbtype.Msg) context.Context {
	return context.WithValue(parent, contextHBRecvMsgQueue, hbRecvQ)
}

// HBSendQ function returns HBSendQ from context
func HBSendQ(ctx context.Context) (hbSendQ chan hbtype.Msg) {
	var ok bool
	hbSendQ, ok = ctx.Value(contextHBSendQueue).(chan hbtype.Msg)
	if ok {
		return
	}
	return nil
}

// WithHBSendQ function returns copy of parent with HBSendQ.
func WithHBSendQ(parent context.Context, HBSendQ chan hbtype.Msg) context.Context {
	return context.WithValue(parent, contextHBSendQueue, HBSendQ)
}

// WithDaemon function returns copy of parent with daemon.
func WithDaemon(parent context.Context, daemon subdaemon.RootManager) context.Context {
	return context.WithValue(parent, contextDaemon, daemon)
}

// Daemon function returns daemon from context
func Daemon(ctx context.Context) subdaemon.RootManager {
	daemon, ok := ctx.Value(contextDaemon).(subdaemon.RootManager)
	if ok {
		return daemon
	}
	panic("unable to retrieve context daemon")
}

// WithUuid function returns copy of parent with uuid.
func WithUuid(parent context.Context, uuid uuid.UUID) context.Context {
	return context.WithValue(parent, contextUuid, uuid)
}

// Uuid function returns uuid from context
func Uuid(ctx context.Context) uuid.UUID {
	id, ok := ctx.Value(contextUuid).(uuid.UUID)
	if ok {
		return id
	}
	return uuid.UUID{}
}

// WithListenAddr function returns copy of parent with uuid.
func WithListenAddr(parent context.Context, addr string) context.Context {
	return context.WithValue(parent, contextListenAddr, addr)
}

// ListenAddr function returns uuid from context
func ListenAddr(ctx context.Context) string {
	id, ok := ctx.Value(contextListenAddr).(string)
	if ok {
		return id
	}
	return ""
}
