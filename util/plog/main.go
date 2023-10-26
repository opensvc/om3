package plog

import (
	"context"
	"fmt"

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

// PkgLogger returns Logger from context with pkg attr set
func PkgLogger(ctx context.Context, pkg string) zerolog.Logger {
	return daemonlogctx.Logger(ctx).With().Str("pkg", pkg).Logger()
}

// GetPkgLogger returns Logger with pkg attr set
func GetPkgLogger(pkg string) zerolog.Logger {
	return log.Logger.With().Str("pkg", pkg).Logger()
}
