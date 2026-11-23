package oxcmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/tmplrender"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	// templateWatcher renders a template again each time something it
	// read changes.
	//
	// It keeps the cluster data the events of an events stream carry, as
	// ox monitor does, the stream replaying the current data first. The
	// stream asks the events of the reads of the last render: a render
	// reading other things opens a new stream, into new cluster data, so
	// none is missed between the two.
	templateWatcher struct {
		cmd *CmdTemplate
		r   *tmplrender.Renderer
		out *templateOutput

		// openStream opens an events stream with replay, asking the
		// events of sub.
		openStream func(ctx context.Context, sub tmplrender.Subscription) (event.ReadCloser, error)

		reads []tmplrender.Read
		sub   tmplrender.Subscription
		st    *tmplrender.State
		cdata *msgbus.ClusterData

		// gen is the generation of the stream: the events and the
		// error of a former one are ignored.
		gen     int
		reader  event.ReadCloser
		evC     chan streamEvent
		errC    chan streamError
		done    chan struct{}
		nextID  uint64
		retries int

		// settling is true while the stream replays: no render until
		// it has been quiet for a while, or a partial range would be
		// written.
		settling bool

		// force asks a render at the next timer, whether a read
		// changed or not.
		force bool

		// dirtySince is the time of the first event not rendered yet,
		// zero when none.
		dirtySince time.Time
		timer      *time.Timer
		reconnect  *time.Timer
	}

	streamEvent struct {
		gen int
		ev  *event.Event
	}

	streamError struct {
		gen int
		err error
	}
)

const (
	reconnectMin = 100 * time.Millisecond
	reconnectMax = 5 * time.Second
)

// settleQuiet is the least quiet period after the stream opens, before the
// replay of the current data is deemed done.
var settleQuiet = 500 * time.Millisecond

func (t *CmdTemplate) watch(r *tmplrender.Renderer, out *templateOutput) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	hupC := make(chan os.Signal, 1)
	signal.Notify(hupC, syscall.SIGHUP)
	defer signal.Stop(hupC)

	c, err := client.New()
	if err != nil {
		return err
	}
	streamClient, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	r.Fetcher = &templateFetcher{c: c}
	w := &templateWatcher{
		cmd: t,
		r:   r,
		out: out,
		openStream: func(ctx context.Context, sub tmplrender.Subscription) (event.ReadCloser, error) {
			getter := streamClient.NewGetEvents().
				SetReplay(true).
				SetRelatives(false).
				SetFilters(sub.Filters()).
				SetSelector(sub.Selector)
			if t.Node != "" {
				getter = getter.SetNodename(t.Node)
			}
			return getter.GetReader(ctx)
		},
	}
	// The first render reads the cluster data of the api: one failing,
	// as on a typo, ends the command.
	data, err := getClusterData(c)
	if err != nil {
		return err
	}
	return w.run(ctx, data, hupC)
}

func (w *templateWatcher) logf(format string, args ...any) {
	w.cmd.logf(format, args...)
}

// run renders the template from data, writes it, and renders it again each
// time a read changes, until ctx is done.
func (w *templateWatcher) run(ctx context.Context, data *clusterdump.Data, hupC <-chan os.Signal) error {
	w.evC = make(chan streamEvent, 1000)
	w.errC = make(chan streamError)
	w.done = make(chan struct{})
	defer close(w.done)
	w.timer = time.NewTimer(time.Hour)
	w.timer.Stop()
	w.reconnect = time.NewTimer(time.Hour)
	w.reconnect.Stop()
	defer w.closeStream()

	w.st = tmplrender.NewState(data)
	res, err := w.r.Render(ctx, w.st)
	if err != nil {
		return err
	}
	w.reads = res.Reads
	w.apply(res)
	w.open(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logf("stopped")
			return nil
		case <-hupC:
			if err := w.cmd.readTemplate(w.r); err != nil {
				w.logf("template not read again, the former is kept: %s", err)
				continue
			}
			w.logf("template read again")
			w.force = true
			if !w.settling {
				// While the stream replays, the render waits for
				// the end of the replay.
				w.arm(0)
			}
		case e := <-w.evC:
			if e.gen != w.gen {
				continue
			}
			w.onEvent(ctx, e.ev)
		case e := <-w.errC:
			if e.gen != w.gen {
				continue
			}
			w.logf("events stream: %s", e.err)
			w.closeStream()
			w.reconnect.Reset(w.backoff())
		case <-w.reconnect.C:
			w.open(ctx)
		case <-w.timer.C:
			w.onTimer(ctx)
		}
	}
}

// backoff returns the delay before the next attempt to open a stream.
func (w *templateWatcher) backoff() time.Duration {
	d := reconnectMin << min(w.retries, 6)
	w.retries++
	return min(d, reconnectMax)
}

// open opens a stream asking the events of the reads, into new cluster data,
// and renders once it replayed the current data. With no event to ask, the
// template reads nothing to watch, and no stream is opened.
func (w *templateWatcher) open(ctx context.Context) {
	w.closeStream()
	w.sub = tmplrender.SubscriptionOf(w.reads)
	if len(w.sub.Kinds) == 0 {
		w.logf("the template reads no cluster data: nothing to watch")
		return
	}
	reader, err := w.openStream(ctx, w.sub)
	if err != nil {
		w.logf("events stream: %s", err)
		w.reconnect.Reset(w.backoff())
		return
	}
	servedBy := ""
	if g, ok := reader.(event.ServedByGetter); ok {
		servedBy = g.GetServedBy()
	}
	w.gen++
	w.reader = reader
	w.nextID = 0
	data := clusterdump.NewData(servedBy)
	w.st = tmplrender.NewState(data)
	w.cdata = msgbus.NewClusterData(data)
	w.settling = true
	w.force = true
	w.dirtySince = time.Now()
	w.arm(w.quiet())
	go func(gen int, reader event.ReadCloser) {
		for {
			ev, err := reader.Read()
			if err != nil {
				select {
				case w.errC <- streamError{gen: gen, err: err}:
				case <-w.done:
				}
				return
			}
			select {
			case w.evC <- streamEvent{gen: gen, ev: ev}:
			case <-w.done:
				return
			}
		}
	}(w.gen, reader)
}

func (w *templateWatcher) closeStream() {
	if w.reader == nil {
		return
	}
	w.gen++
	_ = w.reader.Close()
	w.reader = nil
}

// quiet is the quiet period after an event before a render.
func (w *templateWatcher) quiet() time.Duration {
	if w.settling {
		return max(w.cmd.Delay, settleQuiet)
	}
	return w.cmd.Delay
}

// arm sets the timer to fire after the quiet period d, or at the latest
// MaxDelay after the first event not rendered.
func (w *templateWatcher) arm(d time.Duration) {
	if !w.dirtySince.IsZero() && w.cmd.MaxDelay > 0 {
		if left := time.Until(w.dirtySince.Add(w.cmd.MaxDelay)); left < d {
			d = max(left, 0)
		}
	}
	w.timer.Reset(d)
}

func (w *templateWatcher) onEvent(ctx context.Context, ev *event.Event) {
	if w.nextID != 0 && ev.ID != w.nextID {
		w.logf("events stream: received event %d, expected %d: open it again", ev.ID, w.nextID)
		w.open(ctx)
		return
	}
	w.nextID = ev.ID + 1
	w.retries = 0
	msg, err := msgbus.EventToMessage(*ev)
	if err != nil {
		w.logf("event %d %s: %s", ev.ID, ev.Kind, err)
		return
	}
	if m, ok := msg.(*msgbus.InstanceResourceInfoUpdated); ok {
		w.st.ResourceInfo[tmplrender.ResourceInfoID(m.Path.String(), m.Node)] = m.Checksum
	} else {
		w.cdata.ApplyMessage(msg)
	}
	if w.dirtySince.IsZero() {
		w.dirtySince = time.Now()
	}
	w.arm(w.quiet())
}

func (w *templateWatcher) onTimer(ctx context.Context) {
	w.settling = false
	w.dirtySince = time.Time{}
	changed := tmplrender.Changed(w.reads, w.st)
	if !w.force && len(changed) == 0 {
		return
	}
	w.force = false
	res, err := w.r.Render(ctx, w.st)
	if err == nil {
		w.reads = res.Reads
	} else {
		// A failed render stopped at the error: the reads after it
		// are kept, so the stream keeps the data they read, and the
		// render once the error is gone does not miss it.
		w.reads = mergeReads(w.reads, res.Reads)
	}
	if err != nil {
		w.logf("render failed, %s kept: %s", w.outName(), err)
	} else {
		w.apply(res)
	}
	if sub := tmplrender.SubscriptionOf(w.reads); !sub.Equal(w.sub) {
		w.logf("the template reads other data: open the events stream again")
		w.open(ctx)
	}
}

func (w *templateWatcher) outName() string {
	return w.cmd.Out
}

// apply writes the render, and runs the reload command when the output
// changed. A failure is logged: the watch goes on.
func (w *templateWatcher) apply(res tmplrender.Result) {
	w.cmd.warnMode(w.out, res)
	changed, err := w.out.write(res.Out)
	if err != nil {
		w.logf("write %s: %s", w.outName(), err)
		return
	}
	if !changed {
		return
	}
	w.logf("%s written", w.outName())
	if w.cmd.ReloadCommand == "" {
		return
	}
	// Not cancelled by a stop signal: a reload started finishes, bounded
	// by its timeout.
	if err := runReload(context.Background(), w.cmd.ReloadCommand, w.cmd.Out, w.cmd.ReloadTimeout); err != nil {
		var rerr *reloadError
		if errors.As(err, &rerr) {
			w.logf("%s, the new %s is kept", err, w.outName())
		} else {
			w.logf("reload: %s", err)
		}
		return
	}
	w.logf("reloaded by %s", w.cmd.ReloadCommand)
}

// mergeReads returns the reads of l, the ones of former not in l kept.
func mergeReads(former, l []tmplrender.Read) []tmplrender.Read {
	ids := make(map[string]bool, len(l))
	for _, r := range l {
		ids[r.ID] = true
	}
	merged := append([]tmplrender.Read{}, l...)
	for _, r := range former {
		if !ids[r.ID] {
			merged = append(merged, r)
		}
	}
	return merged
}
