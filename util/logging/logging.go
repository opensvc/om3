package logging

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime/debug"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config is the configuration of the zerolog logger and writers
type Config struct {
	// Enable console logging
	WithConsoleLog bool

	// Enable console logging coloring
	WithColor bool

	// EncodeLogsAsJSON makes the log framework log JSON
	EncodeLogsAsJSON bool

	// WithLogFile makes the framework log to a file
	// the fields below can be skipped if this value is false!
	WithLogFile bool

	// WithSessionLogFile logs to a per-sessionid-fqdn file, for sending to the collector
	WithSessionLogFile bool

	// Directory to log to to when filelogging is enabled
	Directory string

	// Filename is the name of the logfile which will be placed inside the directory
	Filename string

	// MaxSize the max size in MB of the logfile before it's rolled
	MaxSize int

	// MaxBackups the max number of rolled files to keep
	MaxBackups int

	// MaxAge the max age in days to keep a logfile
	MaxAge int
}

// Logger is the opensvc specific zerolog logger
type Logger struct {
	*zerolog.Logger
}

const (
	TimeFormat = "15:04:05.000"
)

var (
	// WithCaller adds the file:line information of the logger caller
	WithCaller bool

	consoleWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: TimeFormat}
)

func init() {
	zerolog.ErrorStackMarshaler = marshalStack
}

func marshalStack(err error) interface{} {
	if !WithCaller {
		return nil
	}
	s := fmt.Sprintf("%+v", err)
	l := strings.Split(s, "\n")
	n := len(l)
	if n < 3 {
		return nil
	}

	f := make([]string, 0)
	for i := 0; i < n-1; i = i + 1 {
		if !strings.HasPrefix(l[i], "\t") || i == 0 {
			continue
		}
		f = append(f, l[i-1]+" "+l[i][1:])
	}
	return f
}

// SetDefaultConsoleWriter set the default console writer
func SetDefaultConsoleWriter(w zerolog.ConsoleWriter) {
	consoleWriter = w
}

// Configure sets up the logging framework
func Configure(config Config) *Logger {
	var writers []io.Writer

	if config.WithConsoleLog {
		consoleWriter.NoColor = !config.WithColor
		writers = append(writers, consoleWriter)
	}
	if config.WithLogFile {
		if fileWriter, err := newRollingFile(config); err == nil {
			writers = append(writers, fileWriter)
		}
	}
	mw := io.MultiWriter(writers...)

	// zerolog.SetGlobalLevel(zerolog.DebugLevel)
	logger := log.Output(mw)

	/*
		logger.Debug().
			Bool("fileLogging", config.FileLoggingEnabled).
			Bool("jsonLogOutput", config.EncodeLogsAsJSON).
			Bool("withCaller", config.WithCaller).
			Str("logDirectory", config.Directory).
			Str("fileName", config.Filename).
			Int("maxSizeMB", config.MaxSize).
			Int("maxBackups", config.MaxBackups).
			Int("maxAgeInDays", config.MaxAge).
			Msg("logging configured")
	*/

	return &Logger{
		Logger: &logger,
	}

}

func newRollingFile(config Config) (io.Writer, error) {
	if err := os.MkdirAll(config.Directory, 0744); err != nil {
		debug.PrintStack()
		log.Error().Err(err).Str("path", config.Directory).Msg("can't create log directory")
		return nil, err
	}

	return &lumberjack.Logger{
		Filename:   path.Join(config.Directory, config.Filename),
		MaxBackups: config.MaxBackups, // files
		MaxSize:    config.MaxSize,    // megabytes
		MaxAge:     config.MaxAge,     // days
	}, nil
}
