package objectlogger

import (
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/logging"
	"github.com/opensvc/om3/util/xsession"
	"github.com/rs/zerolog"
)

type (
	Option func(o *Options)

	Options struct {
		WithColor          bool
		WithConsoleLog     bool
		WithLogFile        bool
		WithSessionLogFile bool
	}
)

func WithColor(s bool) Option {
	return func(o *Options) {
		o.WithColor = s
	}
}

func WithConsoleLog(s bool) Option {
	return func(o *Options) {
		o.WithConsoleLog = s
	}
}

func WithLogFile(s bool) Option {
	return func(o *Options) {
		o.WithLogFile = s
	}
}

func WithSessionLogFile(s bool) Option {
	return func(o *Options) {
		o.WithSessionLogFile = s
	}
}

func New(p naming.Path, opts ...Option) zerolog.Logger {
	options := Options{}
	for _, fopt := range opts {
		fopt(&options)
	}
	config := logging.Config{
		WithColor:          options.WithColor,
		WithConsoleLog:     options.WithConsoleLog,
		WithLogFile:        options.WithLogFile,
		WithSessionLogFile: options.WithSessionLogFile,
		EncodeLogsAsJSON:   true,
		Directory:          p.LogDir(), // contains the ns/kind
		Filename:           p.Name + ".log",
		MaxSize:            5,
		MaxBackups:         1,
		MaxAge:             30,
	}
	logger := logging.Configure(config).With().
		Stringer("o", p).
		Str("n", hostname.Hostname()).
		Stringer("sid", xsession.ID).
		Logger()
	return logger
}
