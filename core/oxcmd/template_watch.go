package oxcmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/tmplrender"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	// templateWatcher renders the templates again each time something they
	// read changes.
	//
	// It keeps the cluster data the events of an events stream carry, as
	// ox monitor does, the stream replaying the current data first. The
	// stream asks the events of the reads of the last renders of all the
	// templates: a render reading other things opens a new stream, into new
	// cluster data, so none is missed between the two.
	templateWatcher struct {
		cmd  *CmdTemplate
		jobs []*templateJob

		// fetcher fetches the values no event carries, for the
		// templates of a config file read again too.
		fetcher tmplrender.Fetcher

		// openStream opens an events stream with replay, asking the
		// events of sub.
		openStream func(ctx context.Context, sub tmplrender.Subscription) (event.ReadCloser, error)

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
		// changed or not, for the reason forceReason.
		force       bool
		forceReason string

		// pending are the events applied since the last render, and
		// replayed the count of the events the stream replayed, kept
		// with --debug only.
		pending  []pendingEvent
		replayed int

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

	// pendingEvent is an event applied, as --debug tells it.
	pendingEvent struct {
		id   uint64
		kind string
		path string
		node string
	}
)

// maxPendingEvents bounds the events kept to tell which led to a render.
const maxPendingEvents = 1000

// maxLoggedEvents bounds the events logged for a render.
const maxLoggedEvents = 20

const (
	reconnectMin = 100 * time.Millisecond
	reconnectMax = 5 * time.Second
)

// settleQuiet is the least quiet period after the stream opens, before the
// replay of the current data is deemed done.
var settleQuiet = 500 * time.Millisecond

// writeRetry is the delay before a template whose file could not be written
// is rendered and written again.
var writeRetry = 5 * time.Second

func (t *CmdTemplate) watch(jobs []*templateJob) error {
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
	fetcher := &templateFetcher{c: c, debugf: t.debugFunc()}
	for _, job := range jobs {
		job.r.Fetcher = fetcher
	}
	w := &templateWatcher{
		cmd:     t,
		jobs:    jobs,
		fetcher: fetcher,
		openStream: func(ctx context.Context, sub tmplrender.Subscription) (event.ReadCloser, error) {
			getter := streamClient.NewGetEvents().
				SetReplay(true).
				SetRelatives(false).
				SetFilters(sub.Filters()).
				SetSelector(sub.Selector)
			if t.Node != "" {
				getter = getter.SetNodename(t.Node)
			}
			t.debugf("api: GET /api/node/name/%s/daemon/event?replay=true filter=%s selector=%q",
				nodeOrLocalhost(t.Node), strings.Join(sub.Filters(), ","), sub.Selector)
			return getter.GetReader(ctx)
		},
	}
	// The first renders read the cluster data of the api: one failing, as
	// on a typo, ends the command.
	data, err := getClusterData(c, t.debugFunc())
	if err != nil {
		return err
	}
	return w.run(ctx, data, hupC)
}

func (w *templateWatcher) logf(format string, args ...any) {
	w.cmd.logf(format, args...)
}

func (w *templateWatcher) debugf(format string, args ...any) {
	w.cmd.debugf(format, args...)
}

func nodeOrLocalhost(node string) string {
	if node == "" {
		return "localhost"
	}
	return node
}

// run renders the templates from data, writes them, and renders a template
// again each time one of its reads changes, until ctx is done.
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
	w.debugf("render: first render, from the cluster status")
	results := make([]tmplrender.Result, len(w.jobs))
	var errs []error
	for i, job := range w.jobs {
		res, err := job.r.Render(ctx, w.st)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results[i] = res
		job.reads = res.Reads
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	retry := w.apply(w.jobs, results)
	w.open(ctx)
	if retry && w.reader == nil {
		w.arm(writeRetry)
	}

	for {
		select {
		case <-ctx.Done():
			w.logf("stopped")
			return nil
		case <-hupC:
			w.readAgain()
			w.force = true
			w.forceReason = "templates read again on SIGHUP"
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
	w.sub = tmplrender.SubscriptionOf(w.reads())
	if len(w.sub.Kinds) == 0 {
		w.logf("the templates read no cluster data: nothing to watch")
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
	if servedBy == "" && w.cmd.Node != "" {
		// A daemon proxying the events of the node asked may not
		// tell the node serving them: it is the node asked. The cluster
		// data keeps the object statuses of the node serving them only.
		servedBy = w.cmd.Node
	}
	if servedBy == "" {
		w.logf("events stream: the node serving the events is not known: the object statuses are not followed")
	}
	w.debugf("events stream: subscribed to %s, selector %q, served by %s",
		strings.Join(w.sub.Kinds, ","), w.sub.Selector, servedBy)
	w.gen++
	w.reader = reader
	w.nextID = 0
	data := clusterdump.NewData(servedBy)
	w.st = tmplrender.NewState(data)
	w.cdata = msgbus.NewClusterData(data)
	w.settling = true
	w.force = true
	w.forceReason = "events stream opened, its replay settled"
	w.pending = nil
	w.replayed = 0
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
	if w.cmd.Debug {
		w.recordEvent(ev)
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

// readAgain reads the templates again, and the config file naming them, on
// SIGHUP. A template not parsing keeps the former, and a config file not read,
// or invalid, keeps the former templates.
//
// The templates of a config file read again replace the former: a template
// of the same out keeps the reads of its last render until it renders again,
// and the file of a template no longer named is left as it is, no longer
// rendered.
func (w *templateWatcher) readAgain() {
	if w.cmd.Config == "" {
		for _, job := range w.jobs {
			if err := job.readTemplate(); err != nil {
				w.logf("template %s not read again, the former is kept: %s", job.in, err)
				continue
			}
			w.logf("template %s read again", job.in)
		}
		return
	}
	jobs, err := w.cmd.prepareConfig()
	if err != nil {
		w.logf("config %s not read again, the former templates are kept: %s", w.cmd.Config, err)
		return
	}
	former := make(map[string]*templateJob, len(w.jobs))
	for _, job := range w.jobs {
		former[job.out] = job
	}
	for _, job := range jobs {
		job.r.Fetcher = w.fetcher
		if f, ok := former[job.out]; ok {
			job.reads = f.reads
			delete(former, job.out)
		}
	}
	for out := range former {
		w.logf("%s is no longer rendered, the file is left as it is", out)
	}
	w.jobs = jobs
	w.logf("config %s read again: %d templates", w.cmd.Config, len(jobs))
}

// reads returns the reads of the last renders of all the templates.
func (w *templateWatcher) reads() []tmplrender.Read {
	var l []tmplrender.Read
	for _, job := range w.jobs {
		l = append(l, job.reads...)
	}
	return l
}

func (w *templateWatcher) onTimer(ctx context.Context) {
	if w.settling {
		w.debugf("events stream: replay settled, %d events applied", w.replayed)
		w.replayed = 0
	}
	w.settling = false
	w.dirtySince = time.Time{}
	if w.cmd.Debug {
		w.logCauses(tmplrender.Changed(w.reads(), w.st))
	}
	w.pending = nil
	force := w.force
	w.force = false

	// The templates whose reads changed are rendered, all of them before
	// a file is written, so a reload command reloading several files
	// runs once, after they are all written.
	var (
		jobs    []*templateJob
		results []tmplrender.Result
		retry   bool
	)
	for _, job := range w.jobs {
		if !force && !job.retry && len(tmplrender.Changed(job.reads, w.st)) == 0 {
			continue
		}
		res, err := job.r.Render(ctx, w.st)
		if err != nil {
			// A failed render stopped at the error: the reads
			// after it are kept, so the stream keeps the data they
			// read, and the render once the error is gone does not
			// miss it. The other templates go on.
			job.reads = mergeReads(job.reads, res.Reads)
			if job.retry || isTransient(err) {
				// No event may come to render it again: an
				// api not answering, or a file to write again.
				job.retry = true
				retry = true
				w.logf("render of %s failed, %s kept, rendered again in %s: %s", job.in, job.out, writeRetry, err)
			} else {
				w.logf("render of %s failed, %s kept: %s", job.in, job.out, err)
			}
			continue
		}
		job.reads = res.Reads
		jobs = append(jobs, job)
		results = append(results, res)
	}
	if sub := tmplrender.SubscriptionOf(w.reads()); !sub.Equal(w.sub) {
		// The renders read data the stream does not serve: rendered
		// from cluster data missing it, as a range of objects not
		// followed yet, a file would be written partial. None is
		// written: the stream is opened again, asking the new reads,
		// and the templates are rendered once it replayed.
		w.logf("the templates read other data: open the events stream again, and render once it replayed")
		w.open(ctx)
		return
	}
	if w.apply(jobs, results) || retry {
		w.arm(writeRetry)
	}
}

// isTransient returns true when a render failed on an api not answering, or
// answering an error of its own, rather than on a value missing: an error no
// event follows the end of.
func isTransient(err error) bool {
	var t *transientError
	return errors.As(err, &t)
}

// apply writes the renders, then runs the reload commands of the files
// changed, each once. A failure is logged: the watch goes on, a file written
// is kept. It returns true when a file could not be written, and is to be
// written again.
func (w *templateWatcher) apply(jobs []*templateJob, results []tmplrender.Result) bool {
	var written []*templateJob
	retry := false
	for i, job := range jobs {
		changed, err := w.cmd.write(job, results[i])
		if err != nil {
			// The reads of the render are recorded already: the
			// file is written again later, or it would stay stale
			// until another read changes.
			w.logf("%s: written again in %s", err, writeRetry)
			job.retry = true
			retry = true
			continue
		}
		job.retry = false
		if changed {
			written = append(written, job)
		}
	}
	if err := w.cmd.reload(written); err != nil {
		for _, e := range unjoin(err) {
			w.logf("%s, the new files are kept", e)
		}
	}
	return retry
}

// unjoin returns the errors err joins, or err.
func unjoin(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	return []error{err}
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

// recordEvent records the event ev applied, to tell which led to the next
// render: during the replay, it is counted only.
func (w *templateWatcher) recordEvent(ev *event.Event) {
	if w.settling {
		w.replayed++
		return
	}
	if len(w.pending) >= maxPendingEvents {
		return
	}
	var data struct {
		Path string `json:"path"`
		Node string `json:"node"`
	}
	_ = json.Unmarshal(ev.Data, &data)
	w.pending = append(w.pending, pendingEvent{id: ev.ID, kind: ev.Kind, path: data.Path, node: data.Node})
}

// logCauses logs why a render is done, or not: the reads changed, and the
// events applied since the last render that may have changed them.
func (w *templateWatcher) logCauses(changed []string) {
	if len(changed) == 0 {
		if w.force {
			w.debugf("render: %s", w.forceReason)
		} else if len(w.pending) > 0 {
			w.debugf("no render: %d events applied, no read changed", len(w.pending))
		}
		return
	}
	if w.force {
		w.debugf("render: %s, and reads changed: %s", w.forceReason, strings.Join(changed, ", "))
	} else {
		w.debugf("render: reads changed: %s", strings.Join(changed, ", "))
	}
	all := w.reads()
	reads := make(map[string]tmplrender.Read, len(all))
	for _, r := range all {
		reads[r.ID] = r
	}
	var causes []pendingEvent
	for _, ev := range w.pending {
		for _, id := range changed {
			if eventConcerns(ev, reads[id]) {
				causes = append(causes, ev)
				break
			}
		}
	}
	for i, ev := range causes {
		if i == maxLoggedEvents {
			w.debugf("render: and %d events more", len(causes)-i)
			break
		}
		at := ev.path
		if ev.node != "" {
			at += "@" + ev.node
		}
		w.debugf("render: caused by event %d %s %s", ev.id, ev.kind, strings.TrimPrefix(at, "@"))
	}
}

// eventConcerns returns true when ev is of a kind the read r follows, of an
// object it reads: the selection of r is matched term by term, a negation
// ignored, which may tell a few events more than the ones selected.
func eventConcerns(ev pendingEvent, r tmplrender.Read) bool {
	if !slices.Contains(r.Kinds, ev.kind) {
		return false
	}
	if r.Selection == "" || r.AllObjects || ev.path == "" {
		return true
	}
	for _, term := range strings.Split(r.Selection, ",") {
		for _, sub := range strings.Split(term, "+") {
			if !strings.HasPrefix(sub, "!") && fnmatch.Match(sub, ev.path, 0) {
				return true
			}
		}
	}
	return false
}
