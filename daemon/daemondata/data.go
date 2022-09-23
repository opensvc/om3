package daemondata

import (
	"context"
	"reflect"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/util/callcount"
	"opensvc.com/opensvc/util/durationlog"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	caller interface {
		call(context.Context, *data)
	}

	data struct {
		// previous holds a deepcopy of pending data just after commit, it
		// is used to publish diff for other nodes
		previous *cluster.Status

		// pending is the live current data (after apply patch, commit local pendingOps)
		pending *cluster.Status

		pendingOps      []jsondelta.Operation // local data pending operations not yet in patchQueue
		patchQueue      patchQueue            // local data patch queue for remotes
		gen             uint64                // gen of local TNodeData
		mergedFromPeer  gens                  // remote dateset gen merged locally
		mergedOnPeer    gens                  // local dataset gen merged remotely
		remotesNeedFull map[string]bool
		localNode       string
		counterCmd      chan<- interface{}
		log             zerolog.Logger
		bus             *pubsub.Bus
	}

	gens       map[string]uint64
	patchQueue map[string]jsondelta.Patch
)

var (
	cmdDurationWarn = time.Second
)

func run(ctx context.Context, cmdC <-chan interface{}) {
	counterCmd, cancel := callcount.Start(ctx, idToName)
	defer cancel()
	d := newData(counterCmd)
	d.log = daemonlogctx.Logger(ctx).With().Str("name", "daemondata").Logger()
	d.log.Info().Msg("starting")
	d.bus = pubsub.BusFromContext(ctx)

	defer d.log.Info().Msg("stopped")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	watchCmd := &durationlog.T{Log: d.log}
	watchDurationCtx, watchDurationCancel := context.WithCancel(context.Background())
	defer watchDurationCancel()
	var beginCmd = make(chan interface{})
	var endCmd = make(chan bool)
	go func() {
		watchCmd.WarnExceeded(watchDurationCtx, beginCmd, endCmd, cmdDurationWarn)
	}()

	for {
		select {
		case <-ctx.Done():
			bg, cleanupCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			go func() {
				d.log.Debug().Msg("drop pending cmds")
				defer cleanupCancel()
				for {
					select {
					case c := <-cmdC:
						dropCmd(ctx, c)
					case <-bg.Done():
						d.log.Debug().Msg("drop pending cmds done")
						return
					}
				}
			}()

			return
		case <-ticker.C:
			d.pending.Monitor.Routines = runtime.NumGoroutine()
		case cmd := <-cmdC:
			if c, ok := cmd.(caller); ok {
				beginCmd <- cmd
				c.call(ctx, d)
				endCmd <- true
			} else {
				d.log.Debug().Msgf("%s{...} is not a caller-interface cmd", reflect.TypeOf(cmd))
				counterCmd <- idUndef
			}
		}
	}
}

type (
	errorSetter interface {
		setError(context.Context, error)
	}

	doneSetter interface {
		setDone(context.Context, bool)
	}

	dataByter interface {
		setDataByte(context.Context, []byte)
	}
)

// dropCmd drops commands with side effects
func dropCmd(ctx context.Context, c interface{}) {
	// TODO implement all side effects
	switch cmd := c.(type) {
	case errorSetter:
		cmd.setError(ctx, nil)
	case doneSetter:
		cmd.setDone(ctx, true)
	case dataByter:
		cmd.setDataByte(ctx, []byte{})
	}
}
