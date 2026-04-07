package plog

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danwakefield/fnmatch"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	Logger struct {
		mu     sync.RWMutex
		logger zerolog.Logger
		prefix string
		q      chan LogMessage

		dropped atomic.Uint64
	}
	LogMessage struct {
		Level     zerolog.Level `json:"level"`
		Message   string        `json:"message"`
		Timestamp time.Time     `json:"time"`
	}
	ctxKey struct{}
)

const (
	levelKey = "LEVEL"

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

func (t *Logger) clone() *Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return &Logger{
		logger: t.logger,
		prefix: t.prefix,
		q:      t.q,
	}
}

func (t *Logger) AddPrefix(prefix string) *Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return &Logger{
		logger: t.logger,
		prefix: t.prefix + prefix,
		q:      t.q,
	}
}

func (t *Logger) WithPrefix(prefix string) *Logger {
	n := t.clone()
	n.prefix = prefix
	return n
}

func (t *Logger) WithQ(q chan LogMessage) *Logger {
	n := t.clone()
	n.q = q
	return n
}

func (t *Logger) Prefix() string {
	return t.prefix
}

func levelToString(level zerolog.Level) string {
	switch level {
	case zerolog.TraceLevel:
		return levelTrace
	case zerolog.DebugLevel:
		return levelDebug
	case zerolog.InfoLevel:
		return levelInfo
	case zerolog.WarnLevel:
		return levelWarn
	case zerolog.ErrorLevel:
		return levelError
	default:
		return level.String()
	}
}

func (t *Logger) Msgf(format string, a ...any) string {
	return fmt.Sprintf(t.prefix+format, a...)
}

func (t *Logger) Infof(format string, a ...any) {
	t.Levelf(zerolog.InfoLevel, format, a...)
}

func (t *Logger) Debugf(format string, a ...any) {
	t.Levelf(zerolog.DebugLevel, format, a...)
}

func (t *Logger) Tracef(format string, a ...any) {
	t.Levelf(zerolog.TraceLevel, format, a...)
}

func (t *Logger) Errorf(format string, a ...any) {
	t.Levelf(zerolog.ErrorLevel, format, a...)
}

func (t *Logger) Warnf(format string, a ...any) {
	t.Levelf(zerolog.WarnLevel, format, a...)
}

func (t *Logger) Levelf(level zerolog.Level, format string, a ...any) {
	msg := t.Msgf(format, a...)

	t.logger.WithLevel(level).Str(levelKey, levelToString(level)).Msg(msg)

	t.sendAudit(level, msg)
}

func (t *Logger) Q() chan LogMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.q
}

func (t *Logger) sendAudit(level zerolog.Level, msg string) {
	t.mu.RLock()
	q := t.q
	t.mu.RUnlock()

	if q == nil {
		return
	}

	lm := LogMessage{Level: level, Message: msg, Timestamp: time.Now()}

	select {
	case q <- lm:
		if n := t.dropped.Swap(0); n > 0 {
			warn := LogMessage{
				Level:     zerolog.WarnLevel,
				Message:   t.Msgf("audit queue saturated: %d message(s) dropped", n),
				Timestamp: time.Now(),
			}
			select {
			case q <- warn:
			case <-time.After(200 * time.Millisecond):
				t.dropped.Add(n)
			}
		}
	default:
		t.dropped.Add(1)
	}
}

func (t *Logger) Attr(k string, v any) *Logger {
	t.mu.RLock()
	logger := t.logger
	prefix := t.prefix
	q := t.q
	t.mu.RUnlock()
	n := &Logger{
		prefix: prefix,
		q:      q,
	}
	switch i := v.(type) {
	case string:
		n.logger = logger.With().Str(k, i).Logger()
	case []string:
		n.logger = logger.With().Strs(k, i).Logger()
	case []byte:
		n.logger = logger.With().Bytes(k, i).Logger()
	case float32:
		n.logger = logger.With().Float32(k, i).Logger()
	case float64:
		n.logger = logger.With().Float64(k, i).Logger()
	case bool:
		n.logger = logger.With().Bool(k, i).Logger()
	case int:
		n.logger = logger.With().Int(k, i).Logger()
	case int32:
		n.logger = logger.With().Int32(k, i).Logger()
	case int64:
		n.logger = logger.With().Int64(k, i).Logger()
	case uint:
		n.logger = logger.With().Uint(k, i).Logger()
	case uint32:
		n.logger = logger.With().Uint32(k, i).Logger()
	case uint64:
		n.logger = logger.With().Uint64(k, i).Logger()
	case time.Duration:
		n.logger = logger.With().Dur(k, i).Logger()
	default:
		n.logger = logger.With().Interface(k, v).Logger()
	}
	return n
}

func (t *Logger) Level(level zerolog.Level) *Logger {
	t.logger = t.logger.Level(level)
	return t
}

func (t *Logger) GetLevel() zerolog.Level {
	return t.logger.GetLevel()
}

func (t *Logger) SetAuditQ(q chan LogMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.q != nil {
		return fmt.Errorf("cannot set audit q: already set")
	}
	t.q = q
	return nil
}

func (t *Logger) UnsetAuditQ(q chan LogMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.q == nil {
		return fmt.Errorf("cannot unset audit q: not set")
	}
	if t.q != q {
		return fmt.Errorf("cannot unset audit q: q does not match")
	}
	t.q = nil
	return nil
}

func (t *Logger) HandleAuditStart(q chan LogMessage, selectedSubsystems []string, labels ...string) {
	if len(selectedSubsystems) != 0 {
		var matched bool
		for _, label := range labels {
			for _, pattern := range selectedSubsystems {
				if matched {
					break
				}
				if fnmatch.Match(pattern, label, 0) {
					matched = true
					break
				}
			}
		}
		if !matched {
			return
		}
	}
	if err := t.SetAuditQ(q); err != nil {
		//t.Debugf("set audit q: %s", err)
		return
	}
	t.Debugf("start auditing")
}

func (t *Logger) HandleAuditStop(q chan LogMessage, selectedSubsystems []string, labels ...string) {
	if len(selectedSubsystems) != 0 {
		var matched bool
		for _, label := range labels {
			for _, pattern := range selectedSubsystems {
				if matched {
					break
				}
				if fnmatch.Match(pattern, label, 0) {
					matched = true
					break
				}
			}
		}
		if !matched {
			return
		}
	}
	if err := t.UnsetAuditQ(q); err != nil {
		//t.Debugf("unset audit q: %s", err)
		return
	}
	t.Debugf("stop auditing")
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
