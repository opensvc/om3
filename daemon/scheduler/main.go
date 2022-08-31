package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/schedule"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		ctx          context.Context
		cancel       context.CancelFunc
		log          zerolog.Logger
		routineTrace routineTracer
		loopDelay    time.Duration

		events  chan any
		delayed delayedMap
	}
	delayedMap   map[string]delayedEntry
	delayedEntry struct {
		Queued   time.Time
		schedule schedule.Entry
		cancel   func()
	}
	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}

	cmdRunDone struct {
		schedule schedule.Entry
		begin    time.Time
		end      time.Time
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay: time.Second,
		log:       log.Logger.With().Str("name", "scheduler").Logger(),
		//m:         make(scheduleMap),
		events:  make(chan any),
		delayed: make(delayedMap),
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("scheduler funcopt.Apply")
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("scheduler"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	return t
}

func entryKey(p path.T, k string) string {
	return fmt.Sprintf("%s:%s", p, k)
}

func (t delayedMap) Add(e schedule.Entry, cancel func()) {
	k := entryKey(e.Path, e.Action)
	t[k] = delayedEntry{
		Queued:   time.Now(),
		schedule: e,
		cancel:   cancel,
	}
}

func (t delayedMap) Del(e schedule.Entry) {
	k := entryKey(e.Path, e.Action)
	delayed, ok := t[k]
	if !ok {
		return
	}
	delayed.cancel()
	delete(t, k)
}

func (t delayedMap) DelPath(p path.T) {
	for _, e := range t {
		if e.schedule.Path != p {
			continue
		}
		t.Del(e.schedule)
	}
}

func (t delayedMap) Purge() {
	for k, e := range t {
		e.cancel()
		delete(t, k)
	}
}

func (t *T) scheduleEntry(e schedule.Entry) {
	now := time.Now() // keep before GetNext call
	next, _, err := e.GetNext()
	if err != nil {
		t.log.Error().Err(err).Str("action", e.Action).Str("definition", e.Definition).Msg("get next")
		t.delayed.Del(e)
		return
	}
	if next.Before(now) {
		t.delayed.Del(e)
		return
	}
	e.Next = next
	delay := next.Sub(now)
	t.log.Info().Str("action", e.Action).Stringer("path", e.Path).Msgf("schedule to run at %s (in %s)", next, delay)
	tmr := time.AfterFunc(delay, func() {
		begin := time.Now()
		t.log.Info().Str("action", e.Action).Stringer("path", e.Path).Msg("run")
		// TODO
		end := time.Now()
		t.events <- cmdRunDone{
			schedule: e,
			begin:    begin,
			end:      end,
		}
	})
	cancel := func() {
		if tmr == nil {
			return
		}
		tmr.Stop()
	}
	t.delayed.Add(e, cancel)
	return
}

func (t *T) MainStart(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)
	started := make(chan error)
	t.Add(1)
	go func() {
		defer t.Done()
		defer t.Trace(t.Name() + "-loop")()
		started <- nil
		t.loop()
	}()
	<-started
	return nil
}

func (t *T) MainStop() error {
	t.cancel()
	t.delayed.Purge()
	return nil
}

func (t *T) loop() {
	t.log.Info().Msg("loop started")
	//daemonData := daemondata.FromContext(t.ctx)
	//daemonData.GetServicePaths()

	relayEvent := func(ev any) {
		t.events <- ev
	}
	bus := pubsub.BusFromContext(t.ctx)
	// TODO: node.conf events
	//defer daemonps.UnSub(bus, daemonps.SubNodeCfg(bus, pubsub.OpUpdate, "scheduler-on-cfg-create", "", relayEvent))
	//defer daemonps.UnSub(bus, daemonps.SubNodeCfg(bus, pubsub.OpDelete, "scheduler-on-cfg-delete", "", relayEvent))
	defer daemonps.UnSub(bus, daemonps.SubCfg(bus, pubsub.OpUpdate, "scheduler-on-cfg-create", "", relayEvent))
	defer daemonps.UnSub(bus, daemonps.SubCfg(bus, pubsub.OpDelete, "scheduler-on-cfg-delete", "", relayEvent))

	for {
		select {
		case ev := <-t.events:
			switch c := ev.(type) {
			case cmdRunDone:
				// cleanup routine
				t.delayed.Del(c.schedule)
				// schedule next run
				c.schedule.Last = c.begin
				t.scheduleEntry(c.schedule)
			case moncmd.CfgUpdated:
				// triggered on daemon start up too
				t.schedule(c.Path)
			case moncmd.CfgDeleted:
				t.unschedule(c.Path)
			default:
				t.log.Error().Interface("cmd", c).Msg("unknown cmd")
			}
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *T) schedule(p path.T) {
	i, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		t.log.Error().Err(err).Msgf("schedule %s", p)
		return
	}
	o, ok := i.(object.Actor)
	if !ok {
		// only actor objects have scheduled actions
		return
	}
	for _, e := range o.PrintSchedule() {
		t.scheduleEntry(e)
	}
}

func (t *T) unschedule(p path.T) {
	t.delayed.DelPath(p)
}
