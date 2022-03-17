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
	contextEventBusCmd = contextKey("eventbus-cmd")
	contextLogger      = contextKey("logger")
	contextUuid        = contextKey("uuid")
)

func (c contextKey) String() string {
	return "daemonctx." + string(c)
}

// EventBusCmd function returns EventBusCmd from context
func EventBusCmd(ctx context.Context) (cmdC chan<- interface{}) {
	var ok bool
	cmdC, ok = ctx.Value(contextEventBusCmd).(chan<- interface{})
	if ok {
		return
	}
	panic("unable to retrieve context EventBusCmd")
}

// WithEventBusCmd function returns copy of parent with eventbus.
func WithEventBusCmd(parent context.Context, evBusCmd chan<- interface{}) context.Context {
	return context.WithValue(parent, contextEventBusCmd, evBusCmd)
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
