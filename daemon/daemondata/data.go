package daemondata

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/util/callcount"
)

type (
	caller interface {
		call(*data)
	}

	data struct {
		current    *cluster.Status
		pending    *cluster.Status
		localNode  string
		counterCmd chan<- interface{}
		log        zerolog.Logger
		eventCmd   chan<- interface{}
	}
)

func run(ctx context.Context, cmdC <-chan interface{}) {
	counterCmd, cancel := callcount.Start(ctx, idToName)
	defer cancel()
	d := newData(counterCmd)
	d.log = daemonctx.Logger(ctx).With().Str("name", "daemon-data").Logger()
	d.log.Info().Msg("starting")
	d.eventCmd = daemonctx.EventBusCmd(ctx)

	defer d.log.Info().Msg("stopped")
	for {
		select {
		case <-ctx.Done():
			cancel()
			return
		case cmd := <-cmdC:
			if c, ok := cmd.(caller); ok {
				c.call(d)
			} else {
				counterCmd <- idUndef
			}
		}
	}
}

func deepCopy(status *cluster.Status) *cluster.Status {
	b, err := json.Marshal(status)
	if err != nil {
		return nil
	}
	newStatus := cluster.Status{}
	if err := json.Unmarshal(b, &newStatus); err != nil {
		return nil
	}
	return &newStatus
}
