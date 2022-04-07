package handlerhelper

import (
	"context"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonctx"
)

type (
	Contexer interface {
		Context() context.Context
	}
	Writer interface {
		Write([]byte) (int, error)
	}
)

func GetWriteAndLog(w Writer, r Contexer, funcName string) (func([]byte) (int, error), zerolog.Logger) {
	log := daemonctx.Logger(r.Context()).With().Str("func", funcName).Logger()
	write := func(b []byte) (int, error) {
		written, err := w.Write(b)
		if err != nil {
			log.Debug().Err(err).Msg(funcName + " write error")
			return written, err
		}
		return written, nil
	}
	return write, log
}
