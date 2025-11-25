package plog

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	Logger struct {
		logger zerolog.Logger
		prefix string
	}
	ctxKey struct{}
)

const (
	levelKey = "OSVC_LOG_LEVEL"

	levelError = "error"
	levelWarn  = "warn"
	levelInfo  = "info"
	levelDebug = "debug"
	levelTrace = "trace"
)

func NewDefaultLogger() *Logger {
	return &Logger{
		logger: log.Logger,
	}
}

func NewLogger(logger zerolog.Logger) *Logger {
	return &Logger{
		logger: logger,
	}
}

func (t *Logger) WithPrefix(prefix string) *Logger {
	t.prefix = prefix
	return t
}

func (t *Logger) Prefix() string {
	return t.prefix
}

func (t *Logger) Msgf(format string, a ...any) string {
	return fmt.Sprintf(t.prefix+format, a...)
}

func (t *Logger) Infof(format string, a ...any) {
	t.logger.Info().Str(levelKey, "6").Msg(t.Msgf(format, a...))
}

func (t *Logger) Debugf(format string, a ...any) {
	t.logger.Debug().Str(levelKey, levelDebug).Msg(t.Msgf(format, a...))
}

func (t *Logger) Tracef(format string, a ...any) {
	t.logger.Debug().Str(levelKey, levelTrace).Msg(t.Msgf(format, a...))
}

func (t *Logger) Errorf(format string, a ...any) {
	t.logger.Error().Str(levelKey, levelError).Msg(t.Msgf(format, a...))
}

func (t *Logger) Warnf(format string, a ...any) {
	t.logger.Warn().Str(levelKey, levelWarn).Msg(t.Msgf(format, a...))
}

func (t *Logger) Levelf(level zerolog.Level, format string, a ...any) {
	switch level {
	case zerolog.TraceLevel:
		t.logger.Debug().Str(levelKey, levelTrace).Msg(t.Msgf(format, a...))
	case zerolog.DebugLevel:
		t.logger.Debug().Str(levelKey, levelDebug).Msg(t.Msgf(format, a...))
	case zerolog.InfoLevel:
		t.logger.Info().Str(levelKey, levelInfo).Msg(t.Msgf(format, a...))
	case zerolog.WarnLevel:
		t.logger.Warn().Str(levelKey, levelWarn).Msg(t.Msgf(format, a...))
	case zerolog.ErrorLevel:
		t.logger.Error().Str(levelKey, levelError).Msg(t.Msgf(format, a...))
	}
}

func (t *Logger) Attr(k string, v any) *Logger {
	logger := Logger{
		logger: t.logger,
		prefix: t.prefix,
	}
	switch i := v.(type) {
	case string:
		logger.logger = t.logger.With().Str(k, i).Logger()
	case []string:
		logger.logger = t.logger.With().Strs(k, i).Logger()
	case []byte:
		logger.logger = t.logger.With().Bytes(k, i).Logger()
	case float32:
		logger.logger = t.logger.With().Float32(k, i).Logger()
	case float64:
		logger.logger = t.logger.With().Float64(k, i).Logger()
	case bool:
		logger.logger = t.logger.With().Bool(k, i).Logger()
	case int:
		logger.logger = t.logger.With().Int(k, i).Logger()
	case int32:
		logger.logger = t.logger.With().Int32(k, i).Logger()
	case int64:
		logger.logger = t.logger.With().Int64(k, i).Logger()
	case uint:
		logger.logger = t.logger.With().Uint(k, i).Logger()
	case uint32:
		logger.logger = t.logger.With().Uint32(k, i).Logger()
	case uint64:
		logger.logger = t.logger.With().Uint64(k, i).Logger()
	case time.Duration:
		logger.logger = t.logger.With().Dur(k, i).Logger()
	default:
		logger.logger = t.logger.With().Interface(k, v).Logger()
	}
	return &logger
}

func (t *Logger) Level(level zerolog.Level) *Logger {
	t.logger = t.logger.Level(level)
	return t
}

func (t *Logger) GetLevel() zerolog.Level {
	return t.logger.GetLevel()
}

func (t *Logger) Logger() zerolog.Logger {
	return t.logger
}

func (t *Logger) WithContext(ctx context.Context) context.Context {
	if lp, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		if lp == t {
			// Do not store same logger.
			return ctx
		}
	}
	return context.WithValue(ctx, ctxKey{}, t)
}

func Ctx(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return l
	}
	return nil
}

type levelWriter struct {
	level  zerolog.Level
	logger zerolog.Logger
}

// Write implements the io.Writer interface.
func (t *levelWriter) Write(p []byte) (n int, err error) {
	event := t.logger.WithLevel(t.level)
	event.Msg(strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

func (t *Logger) Writer(level zerolog.Level) io.Writer {
	return &levelWriter{
		level:  level,
		logger: t.logger,
	}
}
