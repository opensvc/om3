package logging

import (
	"io"
	"os"
	"path"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config is the configuration of the zerolog logger and writers
type Config struct {
	// Enable console logging
	ConsoleLoggingEnabled bool

	// EncodeLogsAsJSON makes the log framework log JSON
	EncodeLogsAsJSON bool

	// FileLoggingEnabled makes the framework log to a file
	// the fields below can be skipped if this value is false!
	FileLoggingEnabled bool

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

	// WithCaller adds the file:line information of the logger caller
	WithCaller bool
}

// Logger is the opensvc specific zerolog logger
type Logger struct {
	*zerolog.Logger
}

var (
	// WithCaller adds the file:line information of the logger caller
	WithCaller bool

	consoleWriter zerolog.ConsoleWriter
)

func init() {
	consoleWriter = zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}
}

// DisableDefaultConsoleWriterColor disable color on defauult console writer
func DisableDefaultConsoleWriterColor() {
	consoleWriter.NoColor = true
}

// SetDefaultConsoleWriter set the default console writer
func SetDefaultConsoleWriter(w zerolog.ConsoleWriter) {
	consoleWriter = w
}

// Configure sets up the logging framework
func Configure(config Config) *Logger {
	var writers []io.Writer

	if config.ConsoleLoggingEnabled {
		writers = append(writers, consoleWriter)
	}
	if config.FileLoggingEnabled {
		if fileWriter, err := newRollingFile(config); err == nil {
			writers = append(writers, fileWriter)
		}
	}
	mw := io.MultiWriter(writers...)

	// zerolog.SetGlobalLevel(zerolog.DebugLevel)
	l := zerolog.New(mw).With().Timestamp()
	if config.WithCaller {
		l = l.Caller()
	}
	logger := l.Logger()

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

	return &Logger{
		Logger: &logger,
	}

}

func newRollingFile(config Config) (io.Writer, error) {
	if err := os.MkdirAll(config.Directory, 0744); err != nil {
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
