package logging

import (
	"fmt"
	stdlog "log"
	"os"

	"github.com/rs/zerolog"
)

// Wraps a zerolog.Logger so its interoperable with Go's standard "log" package

var _ StdLogger = &stdlog.Logger{}

type StdLogger interface {
	Panic(v ...interface{})
	Panicln(v ...interface{})
	Panicf(format string, v ...interface{})
	Fatal(v ...interface{})
	Fatalln(v ...interface{})
	Fatalf(format string, v ...interface{})
	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

func StandardLogger(log zerolog.Logger) StdLogger {
	return &stdLogger{log}
}

type stdLogger struct {
	log zerolog.Logger
}

func (s *stdLogger) Panic(v ...interface{}) {
	s.log.Panic().Msg(fmt.Sprint(v...))
}

func (s *stdLogger) Panicln(v ...interface{}) {
	s.log.Panic().Msg(fmt.Sprintln(v...))
}

func (s *stdLogger) Panicf(format string, v ...interface{}) {
	s.log.Panic().Msg(fmt.Sprintf(format, v...))
}

func (s *stdLogger) Fatal(v ...interface{}) {
	s.log.Fatal().Msg(fmt.Sprint(v...))
	os.Exit(1)
}

func (s *stdLogger) Fatalln(v ...interface{}) {
	s.log.Fatal().Msg(fmt.Sprintln(v...))
	os.Exit(1)
}

func (s *stdLogger) Fatalf(format string, v ...interface{}) {
	s.log.Fatal().Msg(fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (s *stdLogger) Print(v ...interface{}) {
	s.log.Info().Msg(fmt.Sprint(v...))
}

func (s *stdLogger) Println(v ...interface{}) {
	s.log.Info().Msg(fmt.Sprintln(v...))
}

func (s *stdLogger) Printf(format string, v ...interface{}) {
	s.log.Info().Msg(fmt.Sprintf(format, v...))
}
