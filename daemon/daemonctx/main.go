package daemonctx

import (
	"context"

	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/subdaemon"
)

type (
	contextKey string
)

var (
	contextDaemon      = contextKey("daemon")
	contextDaemonData  = contextKey("daemondata-cmd")
	contextHBSendQueue = contextKey("hb-sendQ")
	contextUuid        = contextKey("uuid")
	contextListenAddr  = contextKey("listen-addr")
)

func (c contextKey) String() string {
	return "daemonctx." + string(c)
}

// DaemonDataCmd function returns new DaemonDataCmd from context
func DaemonDataCmd(ctx context.Context) chan<- interface{} {
	cmdC, ok := ctx.Value(contextDaemonData).(chan<- interface{})
	if ok {
		return cmdC
	}
	panic("unable to retrieve context DaemonDataCmd")
}

// HBSendQ function returns HBSendQ from context
func HBSendQ(ctx context.Context) (hbSendQ chan []byte) {
	var ok bool
	hbSendQ, ok = ctx.Value(contextHBSendQueue).(chan []byte)
	if ok {
		return
	}
	return nil
}

// WithDaemonDataCmd function returns copy of parent with daemonCmd.
func WithDaemonDataCmd(parent context.Context, cmd chan<- interface{}) context.Context {
	return context.WithValue(parent, contextDaemonData, cmd)
}

// WithHBSendQ function returns copy of parent with HBSendQ.
func WithHBSendQ(parent context.Context, HBSendQ chan []byte) context.Context {
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
