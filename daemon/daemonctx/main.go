package daemonctx

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/daemon/subdaemon"
)

type (
	// TCtx holds Context and CancelFunc for daemons
	TCtx struct {
		Ctx        context.Context
		CancelFunc context.CancelFunc
	}

	contextKey string
)

var (
	contextDaemon      = contextKey("daemon")
	contextDaemonData  = contextKey("daemondata-cmd")
	contextDaemonPSCmd = contextKey("daemon-pub-sub-cmd")
	contextHBSendQueue = contextKey("hb-sendQ")
	contextLogger      = contextKey("logger")
	contextUuid        = contextKey("uuid")
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

// DaemonPubSubCmd function returns DaemonPubSubCmd from context
func DaemonPubSubCmd(ctx context.Context) (cmdC chan<- interface{}) {
	var ok bool
	cmdC, ok = ctx.Value(contextDaemonPSCmd).(chan<- interface{})
	if ok {
		return
	}
	panic("unable to retrieve context DaemonPubSubCmd")
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

// WithDaemonPubSubCmd function returns copy of parent with daemon pub sub cmd.
func WithDaemonPubSubCmd(parent context.Context, cmd chan<- interface{}) context.Context {
	return context.WithValue(parent, contextDaemonPSCmd, cmd)
}

// WithHBSendQ function returns copy of parent with HBSendQ.
func WithHBSendQ(parent context.Context, HBSendQ chan []byte) context.Context {
	return context.WithValue(parent, contextHBSendQueue, HBSendQ)
}

// WithLogger function returns copy of parent with logger.
func WithLogger(parent context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(parent, contextLogger, logger)
}

// Logger function returns logger from context or returns default logger
func Logger(ctx context.Context) zerolog.Logger {
	logger, ok := ctx.Value(contextLogger).(zerolog.Logger)
	if ok {
		return logger
	}
	return log.Logger
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
