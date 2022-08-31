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
	Panic(v ...any)
	Panicln(v ...any)
	Panicf(format string, v ...any)
	Fatal(v ...any)
	Fatalln(v ...any)
	Fatalf(format string, v ...any)
	Print(v ...any)
	Println(v ...any)
	Printf(format string, v ...any)
}

func StandardLogger(log zerolog.Logger) StdLogger {
	return &stdLogger{log}
}

type stdLogger struct {
	log zerolog.Logger
}

func (s *stdLogger) Panic(v ...any) {
	s.log.Panic().Msg(fmt.Sprint(v...))
}

func (s *stdLogger) Panicln(v ...any) {
	s.log.Panic().Msg(fmt.Sprintln(v...))
}

func (s *stdLogger) Panicf(format string, v ...any) {
	s.log.Panic().Msg(fmt.Sprintf(format, v...))
}

func (s *stdLogger) Fatal(v ...any) {
	s.log.Fatal().Msg(fmt.Sprint(v...))
	os.Exit(1)
}

func (s *stdLogger) Fatalln(v ...any) {
	s.log.Fatal().Msg(fmt.Sprintln(v...))
	os.Exit(1)
}

func (s *stdLogger) Fatalf(format string, v ...any) {
	s.log.Fatal().Msg(fmt.Sprintf(format, v...))
	os.Exit(1)
}

func (s *stdLogger) Print(v ...any) {
	s.log.Info().Msg(fmt.Sprint(v...))
}

func (s *stdLogger) Println(v ...any) {
	s.log.Info().Msg(fmt.Sprintln(v...))
}

func (s *stdLogger) Printf(format string, v ...any) {
	s.log.Info().Msg(fmt.Sprintf(format, v...))
}
