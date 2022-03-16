package daemonctx

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/daemon/subdaemon"
)

type (
	contextKey string
)

var (
	contextDaemon = contextKey("daemon")
	contextLogger = contextKey("logger")
)

func (c contextKey) String() string {
	return "daemonctx." + string(c)
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
