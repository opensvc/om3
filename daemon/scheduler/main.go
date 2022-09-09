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
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
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
		databus      *daemondata.T

		events  chan any
		jobs    Jobs
		enabled bool
	}
	Jobs map[string]Job
	Job  struct {
		Queued   time.Time
		schedule schedule.Entry
		cancel   func()
	}
	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}

	eventJobDone struct {
		schedule schedule.Entry
		begin    time.Time
		end      time.Time
		err      error
	}
)

var (
	skipActionIfUnprovisionned = map[string]bool{
		"sync_all":         true,
		"compliance_auto":  true,
		"resource_monitor": true,
		"run":              true,
	}
	incompatibleNodeMonitorStatus = map[string]bool{
		"init":        true,
		"upgrade":     true,
		"shutting":    true,
		"maintenance": true,
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		log:    log.Logger.With().Str("name", "scheduler").Logger(),
		events: make(chan any),
		jobs:   make(Jobs),
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

func entryKey(e schedule.Entry) string {
	return fmt.Sprintf("%s:%s", e.Path, e.Key)
}

func (t Jobs) Add(e schedule.Entry, cancel func()) {
	k := entryKey(e)
	t[k] = Job{
		Queued:   time.Now(),
		schedule: e,
		cancel:   cancel,
	}
}

func (t Jobs) Del(e schedule.Entry) {
	k := entryKey(e)
	job, ok := t[k]
	if !ok {
		return
	}
	job.cancel()
	delete(t, k)
}

func (t Jobs) DelPath(p path.T) {
	for _, e := range t {
		if e.schedule.Path != p {
			continue
		}
		t.Del(e.schedule)
	}
}

func (t Jobs) Purge() {
	for k, e := range t {
		e.cancel()
		delete(t, k)
	}
}

func (t *T) createJob(e schedule.Entry) {
	// clean up the existing job
	t.jobs.Del(e)

	if !t.enabled {
		return
	}

	log := t.log.With().Str("action", e.Action).Stringer("path", e.Path).Str("key", e.Key).Logger()

	now := time.Now() // keep before GetNext call
	next, _, err := e.GetNext()
	if err != nil {
		log.Error().Err(err).Str("definition", e.Definition).Msg("get next")
		t.jobs.Del(e)
		return
	}
	if next.Before(now) {
		t.jobs.Del(e)
		return
	}
	e.Next = next
	delay := next.Sub(now)
	log.Info().Msgf("schedule to run at %s (in %s)", next, delay)
	tmr := time.AfterFunc(delay, func() {
		begin := time.Now()
		if begin.Sub(next) < 500*time.Millisecond {
			// prevent drift if the gap is small
			begin = next
		}
		err := t.action(e)

		// remember last run, to not run the job too soon after a daemon restart
		if err := e.SetLastRun(begin); err != nil {
			log.Error().Err(err).Msg("update last run failed")
		}

		// remember last success, for users benefit
		if err == nil {
			if err := e.SetLastSuccess(begin); err != nil {
				log.Error().Err(err).Msg("update last success failed")
			}
		}

		// store end time, for duration sampling
		end := time.Now()

		t.events <- eventJobDone{
			schedule: e,
			begin:    begin,
			end:      end,
			err:      err,
		}
	})
	cancel := func() {
		if tmr == nil {
			return
		}
		tmr.Stop()
	}
	t.jobs.Add(e, cancel)
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
	t.jobs.Purge()
	return nil
}

func (t *T) loop() {
	t.log.Info().Msg("loop started")

	relayEvent := func(ev any) {
		t.events <- ev
	}
	t.databus = daemondata.FromContext(t.ctx)
	bus := pubsub.BusFromContext(t.ctx)
	defer daemonps.UnSub(bus, daemonps.SubInstStatus(bus, pubsub.OpUpdate, "scheduler-on-inst-status-update", "", relayEvent))
	defer daemonps.UnSub(bus, daemonps.SubInstStatus(bus, pubsub.OpDelete, "scheduler-on-inst-status-delete", "", relayEvent))
	defer daemonps.UnSub(bus, daemonps.SubNmon(bus, pubsub.OpUpdate, "scheduler-on-nmon-update", relayEvent))

	for {
		select {
		case ev := <-t.events:
			switch c := ev.(type) {
			case eventJobDone:
				// remember last run
				c.schedule.Last = c.begin
				// reschedule
				t.createJob(c.schedule)
			case moncmd.InstStatusDeleted:
				t.onInstStatusDeleted(c)
			case moncmd.InstStatusUpdated:
				t.onInstStatusUpdated(c)
			case moncmd.NmonUpdated:
				t.onNmonUpdated(c)
			default:
				t.log.Error().Interface("cmd", c).Msg("unknown cmd")
			}
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *T) onInstStatusDeleted(c moncmd.InstStatusDeleted) {
	if c.Node != hostname.Hostname() {
		// discard peer node events
		return
	}
	t.log.Info().Stringer("path", c.Path).Msgf("unschedule (instance deleted)")
	t.unschedule(c.Path)
}

func (t *T) onInstStatusUpdated(c moncmd.InstStatusUpdated) {
	if c.Node != hostname.Hostname() {
		// discard peer node events
		return
	}
	provisioned := c.Status.Provisioned.Bool()
	hasAnyJob := t.hasAnyJob(c.Path)
	switch {
	case provisioned:
		t.schedule(c.Path)
	case !provisioned && hasAnyJob:
		t.log.Info().Stringer("path", c.Path).Msgf("unschedule (instance no longer provisionned)")
		t.unschedule(c.Path)
	}
}

func (t *T) onNmonUpdated(c moncmd.NmonUpdated) {
	if c.Node != hostname.Hostname() {
		// discard peer node events
		return
	}
	_, incompatible := incompatibleNodeMonitorStatus[c.Monitor.Status]
	switch {
	case incompatible && t.enabled:
		t.log.Info().Msgf("disable scheduling (node monitor status is now %s)", c.Monitor.Status)
		t.jobs.Purge()
		t.enabled = false
	case !incompatible && !t.enabled:
		t.log.Info().Msgf("enable scheduling (node monitor status is now %s)", c.Monitor.Status)
		t.enabled = true
		t.scheduleAll()
	}
}

func (t *T) hasAnyJob(p path.T) bool {
	for _, job := range t.jobs {
		if job.schedule.Path == p {
			return true
		}
	}
	return false
}

func (t *T) scheduleAll() {
	for _, p := range t.databus.GetServicePaths() {
		t.scheduleObject(p)
	}
	t.scheduleNode()
}

func (t *T) schedule(p path.T) {
	if !t.enabled {
		return
	}
	if p.IsZero() {
		t.scheduleNode()
	} else {
		t.scheduleObject(p)
	}
}

func (t *T) scheduleNode() {
	o, err := object.NewNode()
	if err != nil {
		t.log.Error().Err(err).Msg("schedule node")
		return
	}
	for _, e := range o.PrintSchedule() {
		t.createJob(e)
	}
}

func (t *T) scheduleObject(p path.T) {
	i, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		t.log.Error().Err(err).Msgf("schedule object %s", p)
		return
	}
	o, ok := i.(object.Actor)
	if !ok {
		// only actor objects have scheduled actions
		return
	}
	t.log.Info().Msgf("schedule object %s", p)
	for _, e := range o.PrintSchedule() {
		t.createJob(e)
	}
}

func (t *T) unschedule(p path.T) {
	t.jobs.DelPath(p)
}
