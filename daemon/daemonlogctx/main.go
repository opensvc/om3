package daemonlogctx

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	contextKey string
)

var (
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
	if ctx == nil {
		return log.Logger
	}
	logger, ok := ctx.Value(contextLogger).(zerolog.Logger)
	if ok {
		return logger
	}
	return log.Logger
}
