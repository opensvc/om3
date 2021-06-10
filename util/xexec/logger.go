package xexec

import "github.com/rs/zerolog"

type LoggerExec struct {
	Log      *zerolog.Logger
	LevelOut zerolog.Level
	LevelErr zerolog.Level
}

func NewLoggerExec(log *zerolog.Logger, levelOut, levelErr zerolog.Level) *LoggerExec {
	return &LoggerExec{
		Log:      log,
		LevelOut: levelOut,
		LevelErr: levelErr,
	}
}

func (w LoggerExec) DoOut(s Bytetexter, pid int) {
	w.Log.WithLevel(w.LevelOut).Str("out", s.Text()).Int("pid", pid).Send()
}

func (w LoggerExec) DoErr(s Bytetexter, pid int) {
	w.Log.WithLevel(w.LevelErr).Str("err", s.Text()).Int("pid", pid).Send()
}
