package daemonctx

import (
	"context"

	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	contextKey string
)

var (
	contextDaemon          = contextKey("daemon")
	contextDaemonData      = contextKey("daemondata-cmd")
	contextDaemonPubSubBus = contextKey("daemon-pub-sub-bus")
	contextHBSendQueue     = contextKey("hb-sendQ")
	contextUuid            = contextKey("uuid")
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

// DaemonPubSubBus function returns DaemonPubSubBus from context
func DaemonPubSubBus(ctx context.Context) *pubsub.Bus {
	bus, ok := ctx.Value(contextDaemonPubSubBus).(*pubsub.Bus)
	if ok {
		return bus
	}
	panic("unable to retrieve context DaemonPubSubBus")
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

// WithDaemonPubSubBus function returns copy of parent with daemon pub sub cmd.
func WithDaemonPubSubBus(parent context.Context, bus *pubsub.Bus) context.Context {
	return context.WithValue(parent, contextDaemonPubSubBus, bus)
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
