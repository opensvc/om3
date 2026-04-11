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

		// q is the audit queue, it is created from the head logger q.
		q chan LogMessage

		dropped atomic.Uint64

		// head is the last logger in the chain.
		// it is used for the queue and for cloning the logger chain.
		head *Logger
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
	l := &Logger{
		logger: log.Logger,
	}
	l.head = l
	return l
}

func NewLogger(logger zerolog.Logger) *Logger {
	l := &Logger{
		logger: logger,
	}
	l.head = l
	return l
}

func (t *Logger) clone() *Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()

	l := &Logger{
		logger: t.logger,
		prefix: t.prefix,
		head:   t.head,
	}
	if t.head != nil {
		l.q = t.head.q
	} else {
		// unexpected
		l.q = t.q
		l.head = t
	}

	return l
}

func (t *Logger) AddPrefix(prefix string) *Logger {
	l := t.clone()
	l.prefix = t.prefix + prefix
	return l
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
	n := t.clone()
	switch i := v.(type) {
	case string:
		n.logger = n.logger.With().Str(k, i).Logger()
	case []string:
		n.logger = n.logger.With().Strs(k, i).Logger()
	case []byte:
		n.logger = n.logger.With().Bytes(k, i).Logger()
	case float32:
		n.logger = n.logger.With().Float32(k, i).Logger()
	case float64:
		n.logger = n.logger.With().Float64(k, i).Logger()
	case bool:
		n.logger = n.logger.With().Bool(k, i).Logger()
	case int:
		n.logger = n.logger.With().Int(k, i).Logger()
	case int32:
		n.logger = n.logger.With().Int32(k, i).Logger()
	case int64:
		n.logger = n.logger.With().Int64(k, i).Logger()
	case uint:
		n.logger = n.logger.With().Uint(k, i).Logger()
	case uint32:
		n.logger = n.logger.With().Uint32(k, i).Logger()
	case uint64:
		n.logger = n.logger.With().Uint64(k, i).Logger()
	case time.Duration:
		n.logger = n.logger.With().Dur(k, i).Logger()
	default:
		n.logger = n.logger.With().Interface(k, v).Logger()
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
	t.q = q
	if t.head != nil {
		t.head.q = q
	} else {
		// unexpected
		t.head = t
	}
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
	if t.head != nil {
		t.head.q = nil
	} else {
		// unexpected
		t.head = t
	}
	return nil
}

func (t *Logger) HandleAuditStart(q chan LogMessage, selectedSubsystems []string, labels ...string) {
	if !t.auditMatch(selectedSubsystems, labels...) {
		return
	}
	if err := t.SetAuditQ(q); err != nil {
		return
	}
	t.Debugf("start auditing")
}

func (t *Logger) HandleAuditStop(q chan LogMessage, selectedSubsystems []string, labels ...string) {
	if !t.auditMatch(selectedSubsystems, labels...) {
		return
	}
	if err := t.UnsetAuditQ(q); err != nil {
		return
	}
	t.Debugf("stop auditing")
}

func (t *Logger) auditMatch(selectedSubsystems []string, labels ...string) bool {
	if len(selectedSubsystems) == 0 {
		return true
	}

	for _, label := range labels {
		for _, pattern := range selectedSubsystems {
			if fnmatch.Match(pattern, label, 0) {
				return true
			}
		}
	}
	return false
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
