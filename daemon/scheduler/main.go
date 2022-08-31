package scheduler

import (
	"context"
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

		m scheduleMap
	}
	scheduleMap   map[string]schedule.Table
	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay: time.Second,
		log:       log.Logger.With().Str("name", "scheduler").Logger(),
		m:         make(scheduleMap),
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
	return nil
}

func (t *T) loop() {
	t.log.Info().Msg("loop started")
	//daemonData := daemondata.FromContext(t.ctx)
	//daemonData.GetServicePaths()

	events := make(chan any)
	relayEvent := func(ev any) {
		events <- ev
	}
	bus := pubsub.BusFromContext(t.ctx)
	// TODO
	//defer daemonps.UnSub(bus, daemonps.SubNodeCfg(bus, pubsub.OpUpdate, "scheduler-on-cfg-create", "", relayEvent))
	//defer daemonps.UnSub(bus, daemonps.SubNodeCfg(bus, pubsub.OpDelete, "scheduler-on-cfg-delete", "", relayEvent))
	defer daemonps.UnSub(bus, daemonps.SubCfg(bus, pubsub.OpUpdate, "scheduler-on-cfg-create", "", relayEvent))
	defer daemonps.UnSub(bus, daemonps.SubCfg(bus, pubsub.OpDelete, "scheduler-on-cfg-delete", "", relayEvent))

	for {
		select {
		case ev := <-events:
			switch c := ev.(type) {
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
	ps := p.String()
	t.m[ps] = o.PrintSchedule()
	for _, e := range o.PrintSchedule() {
		// TODO
		e.Next = time.Time{}
		t.log.Info().Msgf("schedule %s %s", ps, e.Key)
	}
}

func (t *T) unschedule(p path.T) {
	ps := p.String()
	delete(t.m, ps)
}
