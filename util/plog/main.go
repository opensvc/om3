package plog

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/daemon/daemonlogctx"
)

type (
	Logger struct {
		zerolog.Logger
		Prefix string
	}
)

func (t *Logger) Msgf(format string, a ...any) string {
	return fmt.Sprintf(t.Prefix+format, a...)
}

func (t *Logger) Infof(format string, a ...any) {
	t.Info().Msg(t.Msgf(format, a...))
}

func (t *Logger) Debugf(format string, a ...any) {
	t.Debug().Msg(t.Msgf(format, a...))
}

func (t *Logger) Errorf(format string, a ...any) {
	t.Error().Msg(t.Msgf(format, a...))
}

func (t *Logger) Warnf(format string, a ...any) {
	t.Warn().Msg(t.Msgf(format, a...))
}

func (t *Logger) Attr(k string, v any) *Logger {
	logger := Logger{
		Logger: t.Logger,
		Prefix: t.Prefix,
	}
	switch i := v.(type) {
	case string:
		logger.Logger = t.Logger.With().Str(k, i).Logger()
	case []string:
		logger.Logger = t.Logger.With().Strs(k, i).Logger()
	case []byte:
		logger.Logger = t.Logger.With().Bytes(k, i).Logger()
	case float32:
		logger.Logger = t.Logger.With().Float32(k, i).Logger()
	case float64:
		logger.Logger = t.Logger.With().Float64(k, i).Logger()
	case int:
		logger.Logger = t.Logger.With().Int(k, i).Logger()
	case int32:
		logger.Logger = t.Logger.With().Int32(k, i).Logger()
	case int64:
		logger.Logger = t.Logger.With().Int64(k, i).Logger()
	case uint:
		logger.Logger = t.Logger.With().Uint(k, i).Logger()
	case uint32:
		logger.Logger = t.Logger.With().Uint32(k, i).Logger()
	case uint64:
		logger.Logger = t.Logger.With().Uint64(k, i).Logger()
	case time.Duration:
		logger.Logger = t.Logger.With().Dur(k, i).Logger()
	default:
		logger.Logger = t.Logger.With().Interface(k, v).Logger()
	}
	return &logger
}

// PkgLogger returns Logger from context with pkg attr set
func PkgLogger(ctx context.Context, pkg string) zerolog.Logger {
	return daemonlogctx.Logger(ctx).With().Str("pkg", pkg).Logger()
}

// GetPkgLogger returns Logger with pkg attr set
func GetPkgLogger(pkg string) zerolog.Logger {
	return log.Logger.With().Str("pkg", pkg).Logger()
}
