package logging

import (
	"fmt"
	"io"
	"os"
	"os/exec"
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
type Config struct {
	// WithCaller includes the caller file:line to the records
	WithCaller bool

	// Enable console logging
	WithConsoleLog bool

	// Enable console logging coloring
	WithColor bool

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

// Logger is the opensvc specific zerolog logger
const (
	TimeFormat = "15:04:05.000"
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

	if _, err := exec.LookPath("journalctl"); err == nil {
		if writer := journald.NewJournalDWriter(); writer != nil {
			writers = append(writers, writer)
		}
	}
	if config.WithConsoleLog {
		consoleWriter := zerolog.ConsoleWriter{
			Out:              os.Stderr,
			TimeFormat:       TimeFormat,
			NoColor:          !config.WithColor,
			FormatLevel:      FormatLevel,
			FormatFieldName:  func(i any) string { return "" },
			FormatFieldValue: func(i any) string { return "" },
			FormatMessage: func(i any) string {
				if s, ok := i.(string); ok {
					return strings.TrimPrefix(s, "instance: ")
				} else {
					return ""
				}
			},
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
	mw := io.MultiWriter(writers...)

	logger := log.Output(mw)

	if config.WithCaller {
		// skip one more for plog wrappers
		logger = logger.With().CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount + 1).Logger()
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
