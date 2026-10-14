package logging

import (
	"fmt"
	"io"
	"log/syslog"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config is the configuration of the zerolog logger and writers
type (
	Config struct {
		// WithCaller includes the caller file:line to the records
		WithCaller bool

		// Enable console logging
		WithConsoleLog bool

		// Enable console logging coloring
		WithColor bool

		// ConsoleFormat is the format of the console records:
		// ConsoleFormatFull, the default, or ConsoleFormatPlain.
		ConsoleFormat string

		WithJournald bool
		WithSyslogd  bool

		// LogFile makes the framework log to a file
		LogFile string

		// SessionLogFile logs to a per-sessionid file
		SessionLogFile string

		// Level is the minimum log record level to accept.
		// debug, info, warn[ing], error, fatal, panic
		Level string

		// MaxSize the max size in MB of the logfile before it's rolled
		MaxSize int

		// MaxBackups the max number of rolled files to keep
		MaxBackups int

		// MaxAge the max age in days to keep a logfile
		MaxAge int
	}

	minLevelWriter struct {
		w   io.Writer
		min zerolog.Level
	}
)

// Logger is the opensvc specific zerolog logger
const (
	TimeFormat = "15:04:05.000"

	// ConsoleFormatFull shows a console record as its time, its level and
	// its message.
	ConsoleFormatFull = "full"

	// ConsoleFormatPlain shows a console record as its message alone, as
	// the lines a command prints, with its level before it from the
	// warnings up, for a command whose output tells a story its records
	// are lines of.
	ConsoleFormatPlain = "plain"
)

var (
	// WithCaller adds the file:line information of the logger caller
	WithCaller bool
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

// DropFields is the FormatPrepare of a console writer showing an entry as
// its time, its level, its message and its error: it removes the other
// fields, which the writer separates by a space each, even when it renders
// them as nothing, padding the line with as many trailing blanks.
func DropFields(evt map[string]any) error {
	for k := range evt {
		switch k {
		case zerolog.TimestampFieldName, zerolog.LevelFieldName, zerolog.MessageFieldName, zerolog.CallerFieldName, zerolog.ErrorFieldName:
		default:
			delete(evt, k)
		}
	}
	return nil
}

func FormatLevel(i interface{}) string {
	var l string
	if ll, ok := i.(string); ok {
		switch ll {
		case "trace":
			l = color.New(color.FgMagenta).Sprint("TRC")
		case "debug":
			l = color.New(color.FgYellow).Sprint("DBG")
		case "info":
			l = color.New(color.FgGreen).Sprint("INF")
		case "warn":
			l = color.New(color.FgHiYellow).Sprint("WRN")
		case "error":
			l = color.New(color.FgRed, color.Bold).Sprint("ERR")
		case "fatal":
			l = color.New(color.FgRed, color.Bold).Sprint("FTL")
		case "panic":
			l = color.New(color.FgRed, color.Bold).Sprint("PNC")
		default:
			l = color.New(color.Bold).Sprint("???")
		}
	} else {
		if i == nil {
			l = color.New(color.Bold).Sprint("???")
		} else {
			l = strings.ToUpper(fmt.Sprintf("%s", i))[0:3]
		}
	}
	return l
}

// FormatPlainLevel is the level of a record in the plain console format:
// none below the warnings, which read as the lines a command prints.
func FormatPlainLevel(i interface{}) string {
	if s, ok := i.(string); ok {
		switch s {
		case "trace", "debug", "info":
			return ""
		}
	}
	return FormatLevel(i)
}

// Configure sets up the logging framework
func Configure(config Config) error {
	var writers []io.Writer

	if config.Level == "none" {
		zerolog.SetGlobalLevel(zerolog.NoLevel)
	} else if config.Level == "" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else if configLevel, err := zerolog.ParseLevel(config.Level); err != nil {
		return fmt.Errorf("invalid log level %s", config.Level)
	} else {
		zerolog.SetGlobalLevel(configLevel)
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano

	if config.WithJournald && journalEnabled() {
		if writer := journald.NewJournalDWriter(); writer != nil {
			writers = append(writers, minLevelWriter{
				w:   writer,
				min: zerolog.InfoLevel,
			})
		}
	} else if config.WithSyslogd {
		if writer, err := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "om"); err == nil {
			writers = append(writers, writer)
		}
	}

	if config.WithConsoleLog {
		consoleWriter := zerolog.ConsoleWriter{
			Out:           os.Stderr,
			TimeFormat:    TimeFormat,
			NoColor:       !config.WithColor,
			FormatLevel:   FormatLevel,
			FormatPrepare: DropFields,
			FormatMessage: func(i any) string {
				if s, ok := i.(string); ok {
					return strings.TrimPrefix(s, "instance: ")
				} else {
					return ""
				}
			},
		}
		if config.ConsoleFormat == ConsoleFormatPlain {
			consoleWriter.PartsOrder = []string{zerolog.LevelFieldName, zerolog.MessageFieldName}
			consoleWriter.FormatLevel = FormatPlainLevel
		}
		writers = append(writers, consoleWriter)
	}
	if config.SessionLogFile != "" {
		if fileWriter, err := newSessionLogFile(config.SessionLogFile); err != nil {
			return err
		} else {
			writers = append(writers, fileWriter)
		}
	}

	mw := zerolog.MultiLevelWriter(writers...)

	logger := log.Output(mw)

	if config.WithCaller {
		// skip one more for plog wrappers
		logger = logger.With().CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount + 2).Logger()
	}

	log.Logger = logger
	return nil
}

func newRollingFile(config Config) (io.Writer, error) {
	directory := filepath.Dir(config.LogFile)
	if err := os.MkdirAll(directory, 0744); err != nil {
		debug.PrintStack()
		log.Error().Err(err).Str("path", directory).Msg("can't create log directory")
		return nil, err
	}

	return &lumberjack.Logger{
		Filename:   path.Join(config.LogFile),
		MaxBackups: config.MaxBackups, // files
		MaxSize:    config.MaxSize,    // megabytes
		MaxAge:     config.MaxAge,     // days
	}, nil
}

func (t minLevelWriter) Write(p []byte) (int, error) {
	return t.w.Write(p)
}

func (t minLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if level < t.min {
		return len(p), nil
	}
	if lw, ok := t.w.(zerolog.LevelWriter); ok {
		return lw.WriteLevel(level, p)
	}
	return t.w.Write(p)
}
