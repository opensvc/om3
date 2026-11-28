// Package prefetch pulls the images of the container resources of the node
// ahead of their start, so a failover to the node finds them in the store of
// the engine, and neither waits for the registry nor fails with it.
//
// It is a cache, kept on a best effort: an image is pulled when the store
// does not hold it, and never pulled again once there. What to hold is read
// from the instance configs of the node, which say the image each container
// resource would run, with the engine and the store it runs from: a Pull.
// The resources saying the same Pull, whatever their objects, are one key,
// pulled once.
//
// It works from events, and polls nothing:
//
//   - a key new to the node is checked, a while after, at a time drawn at
//     random so the nodes of a cluster receiving the same configuration do
//     not pull together;
//   - a failure is tried again later, less and less often, and at once when
//     the configuration of an object of the key changes, which may be its
//     fix, or when a container of such an object turns up on another node,
//     which then holds an image to copy;
//   - an image untagged or removed from the store of an engine, which the
//     container monitor tells, has the keys of the engine checked again: the
//     engines do not name the image removed in a way to rely on, docker
//     telling its id alone, so all are checked, once for a burst of
//     removals, as an image prune makes.
//
// A key is checked by asking the store of its engine, which the daemon does
// itself: it reads no credentials and changes nothing. A missing image is
// fetched by the command of an object of the key, "om <path> instance
// prefetch", which copies it from a node of the object holding it, and
// pulls it from its registry when none does. The command is what knows the
// nodes and the registry credentials of the object, and the user of a
// rootless store: the daemon handles none of them. The store of
// such a user is not one the daemon reads, nor one the container monitor
// tells the removals of: its keys always go through the command, and are not
// pulled again after a prune.
//
// The command records the outcome of the pull on the node, where the status
// of the resources reads it to warn of a missing image whose prefetch
// failed. The daemon tells the instances when an outcome changes to or from
// failed, for the warning to show and clear as it happens.
//
// One check, copy or pull runs at a time. The delays are counted on the clock
// of the node, and compared to no time of another node: a container of
// another node is up from when this node is told so. A frozen node or
// instance is prefetched as another: nothing of the instance changes.
package prefetch

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/proc"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	// Manager is the daemon component prefetching the images.
	Manager struct {
		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger
		sub    *pubsub.Subscription
		subQS  pubsub.QueueSizer
		wg     sync.WaitGroup

		publisher      pubsub.Publisher
		localhost      string
		labelLocalhost pubsub.Label

		// keys are the images the node is meant to hold.
		keys map[resource.Pull]*entry

		// objects is what each object of the node pulls, by rid: what
		// its last instance config said.
		objects map[naming.Path]map[string]resource.Pull

		// peerUp are the resources of the objects of the node that
		// pull an image, last told up on another node.
		peerUp map[naming.Path]map[peerResource]bool

		// running is the key of the job that runs, nil when none does.
		running *resource.Pull

		// results is where the job that runs tells what it did.
		results chan result

		// sweepAt is when the outcomes recorded for keys the node no
		// longer has are removed, zero once done.
		sweepAt time.Time

		// om is the command the daemon runs the prefetch of an object
		// with: itself.
		om string

		// now is the clock of the node, jitter draws a delay below a
		// duration, check asks the store of an engine, exec runs the
		// prefetch command of an object, and start runs a job apart
		// from the events: all are replaced by the tests.
		now    func() time.Time
		jitter func(time.Duration) time.Duration
		check  func(context.Context, resource.Pull) (present bool, id string, checked bool, err error)
		exec   func(context.Context, naming.Path, string) error
		start  func(job)
	}

	// entry is an image the node is meant to hold.
	entry struct {
		pull resource.Pull

		// refs are the resources pulling the image, by object.
		refs map[naming.Path][]string

		// state is the last outcome known of the key: none, or one
		// of the states of a resource.PrefetchOutcome.
		state string

		// imageID is the id of the image in the store, as last seen.
		imageID string

		// attempts counts the tries that failed in a row.
		attempts int

		// dueAt is when the key is to be checked, zero for a key
		// settled: its image seen in the store, and nothing since
		// saying it may be gone.
		dueAt time.Time
	}

	// peerResource is a resource of an object on another node.
	peerResource struct {
		node string
		rid  string
	}

	// job is a check of a key, and the pull of its image when missing,
	// by the command of the object at path, for its resource rid.
	job struct {
		pull resource.Pull
		path naming.Path
		rid  string
	}

	// result is what a job did.
	result struct {
		pull resource.Pull

		// err is why nothing is known of the image: the engine did
		// not answer, or the command could not run.
		err error

		// present says the store was seen holding the image, id, with
		// nothing to pull.
		present bool
		id      string

		// outcome is what the command recorded, when it ran.
		outcome *resource.PrefetchOutcome
	}
)

const (
	// initialDelay bounds the delay, drawn at random, before the first
	// check of a key.
	initialDelay = time.Minute

	// retryDelay is the delay before the first new try of a key that
	// failed, doubled at each failure in a row, up to maxRetryDelay.
	retryDelay    = 5 * time.Minute
	maxRetryDelay = 4 * time.Hour

	// recheckDelay is how long the removals of images settle before the
	// keys of their engine are checked: an image prune tells many.
	recheckDelay = 5 * time.Second

	// sweepDelay is how long after its start the daemon removes the
	// outcomes recorded for keys the node no longer has: once every
	// object of the node had the time to say what it pulls.
	sweepDelay = 10 * time.Minute

	// checkTimeout bounds the question asked to the store of an engine,
	// and execTimeout the prefetch command, which pulls within the
	// pull_timeout of its resource.
	checkTimeout = time.Minute
	execTimeout  = time.Hour

	// driverTypePrefix is what the container monitor names an engine
	// with, before its name: "container.docker" is docker.
	driverTypePrefix = "container."
)

// NewManager returns the component, not started.
func NewManager(subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	t := &Manager{
		subQS:          subQS,
		om:             os.Args[0],
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/prefetch").WithPrefix("daemon: prefetch: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		keys:           make(map[resource.Pull]*entry),
		objects:        make(map[naming.Path]map[string]resource.Pull),
		peerUp:         make(map[naming.Path]map[peerResource]bool),
		results:        make(chan result, 1),
		now:            time.Now,
		jitter: func(d time.Duration) time.Duration {
			if d <= 0 {
				return 0
			}
			return rand.N(d)
		},
		check: checkStore,
	}
	t.exec = t.execCommand
	t.start = func(j job) {
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			t.results <- t.do(t.ctx, j)
		}()
	}
	return t
}

// Start starts following what the objects of the node pull.
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)
	t.startSubscriptions()
	t.sweepAt = t.now().Add(sweepDelay)

	// The configs published before the subscription, as on a component
	// started late, are followed too.
	for p, cfg := range instance.ConfigData.GetByNode(t.localhost) {
		if cfg != nil {
			t.setObject(p, pullsOf(*cfg))
		}
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.worker()
	}()
	t.log.Infof("started")
	return nil
}

// Stop ends the component, and the command it runs: a pull cut short leaves
// the layers it got in the store, which the next pull does not fetch again.
func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	if t.sub != nil {
		if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
			t.log.Warnf("subscription stop: %s", err)
		}
	}
	t.wg.Wait()
	return nil
}

func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.prefetch", t.subQS)
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.InstanceConfigDeleted{}, t.labelLocalhost)
	// The instances of the other nodes are followed, for their
	// containers turning up.
	sub.AddFilter(&msgbus.InstanceStatusUpdated{})
	sub.AddFilter(&msgbus.InstanceStatusDeleted{})
	sub.AddFilter(&msgbus.ContainerImageRemoved{}, t.labelLocalhost)
	sub.Start()
	t.sub = sub
}

// worker reads the events, the results of the jobs, and the clock, one at a
// time: the keys are touched by nothing else.
func (t *Manager) worker() {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		t.tick()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		var wake <-chan time.Time
		if at := t.nextWake(); !at.IsZero() {
			timer.Reset(max(at.Sub(t.now()), 0))
			wake = timer.C
		}
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			t.onEvent(i)
		case r := <-t.results:
			t.onResult(r)
		case <-wake:
		}
	}
}

func (t *Manager) onEvent(i any) {
	switch c := i.(type) {
	case *msgbus.AuditStart:
		t.log.HandleAuditStart(c.Q, c.Subsystems, "prefetch")
	case *msgbus.AuditStop:
		t.log.HandleAuditStop(c.Q, c.Subsystems, "prefetch")
	case *msgbus.InstanceConfigUpdated:
		if c.Node == t.localhost {
			t.setObject(c.Path, pullsOf(c.Value))
		}
	case *msgbus.InstanceConfigDeleted:
		if c.Node == t.localhost {
			t.setObject(c.Path, nil)
		}
	case *msgbus.InstanceStatusUpdated:
		if c.Node != t.localhost {
			t.onPeerStatus(c.Path, c.Node, c.Value)
		}
	case *msgbus.InstanceStatusDeleted:
		if c.Node != t.localhost {
			t.forgetPeer(c.Path, c.Node)
		}
	case *msgbus.ContainerImageRemoved:
		if c.Node == t.localhost {
			t.onImageRemoved(c)
		}
	}
}

// onPeerStatus follows the instance of an object of the node on another
// node, and has the keys of the object that failed tried again at once when
// a resource pulling an image turns up there: its container runs from the
// image, which the node now holds for the prefetch to copy. It is what
// follows an image loaded by hand on a node, and the object started there.
//
// A resource turns up when told up after not being so, by the node itself:
// no time of the other node is read. One up when this daemon starts is told
// so once, when no key has failed yet. An image loaded on a node and never
// run there turns nothing up: the next try of the backoff finds it.
//
// The key of the job that runs is tried again too, after it: its command
// may have asked the node before the image was there.
func (t *Manager) onPeerStatus(p naming.Path, node string, st instance.Status) {
	pulls, ok := t.objects[p]
	if !ok {
		return
	}
	var turnedUp []string
	for rid := range pulls {
		r := peerResource{node: node, rid: rid}
		up := st.Resources[rid].Status == status.Up
		if up && !t.peerUp[p][r] {
			turnedUp = append(turnedUp, rid)
		}
		if !up {
			delete(t.peerUp[p], r)
			continue
		}
		if t.peerUp[p] == nil {
			t.peerUp[p] = make(map[peerResource]bool)
		}
		t.peerUp[p][r] = true
	}
	if len(turnedUp) == 0 {
		return
	}
	slices.Sort(turnedUp)
	now := t.now()
	for _, pull := range pulls {
		e := t.keys[pull]
		if e == nil || e.attempts == 0 {
			continue
		}
		t.log.Infof("%s: try again, %s of %s is up on %s", pull, strings.Join(turnedUp, ", "), p, node)
		if !t.isRunning(pull) {
			e.attempts = 0
		}
		e.dueAt = now
	}
}

// forgetPeer forgets what is up of an object on a node whose instance is
// gone: one made again there turns its resources up again.
func (t *Manager) forgetPeer(p naming.Path, node string) {
	for r := range t.peerUp[p] {
		if r.node == node {
			delete(t.peerUp[p], r)
		}
	}
}

// pullsOf returns what the resources of an instance config pull, by rid.
func pullsOf(cfg instance.Config) map[string]resource.Pull {
	if cfg.ActorConfig == nil {
		return nil
	}
	m := make(map[string]resource.Pull)
	for rid, rcfg := range cfg.Resources {
		if rcfg.Pull != nil && !rcfg.Pull.IsZero() {
			m[rid] = *rcfg.Pull
		}
	}
	return m
}

// setObject follows what an object of the node pulls now: the keys its
// resources no longer say lose them, the ones no resource says any more are
// dropped with their outcome, and the new ones are checked a while after.
//
// An object already followed whose configuration changes has its keys that
// failed tried again at once: the change may be the fix, as a tag
// corrected names another key, and a secret corrected the same one.
func (t *Manager) setObject(p naming.Path, pulls map[string]resource.Pull) {
	former, known := t.objects[p]
	for rid, pull := range former {
		if pulls[rid] != pull {
			t.unref(pull, p, rid)
		}
	}
	for rid, pull := range pulls {
		if former[rid] != pull {
			t.ref(pull, p, rid)
		}
	}
	if len(pulls) == 0 {
		delete(t.objects, p)
		delete(t.peerUp, p)
		return
	}
	t.objects[p] = pulls
	for r := range t.peerUp[p] {
		if _, ok := pulls[r.rid]; !ok {
			delete(t.peerUp[p], r)
		}
	}
	if !known {
		return
	}
	for _, pull := range pulls {
		if e := t.keys[pull]; e != nil && e.attempts > 0 && !t.isRunning(pull) {
			t.log.Infof("%s: try again, the configuration of %s changed", pull, p)
			e.attempts = 0
			e.dueAt = t.now()
		}
	}
}

func (t *Manager) isRunning(pull resource.Pull) bool {
	return t.running != nil && *t.running == pull
}

// ref adds a resource to the ones pulling an image. A key new to the node
// starts from the outcome recorded for it, as by the daemon before its
// restart, and is checked after a delay drawn at random.
func (t *Manager) ref(pull resource.Pull, p naming.Path, rid string) {
	e := t.keys[pull]
	if e == nil {
		e = &entry{pull: pull, refs: make(map[naming.Path][]string)}
		if outcome, ok := resource.ReadPrefetchOutcome(pull); ok {
			e.state = outcome.State
			e.imageID = outcome.ImageID
		}
		e.dueAt = t.now().Add(t.jitter(initialDelay))
		t.keys[pull] = e
		t.log.Debugf("%s: new, for %s %s", pull, p, rid)
	}
	if !slices.Contains(e.refs[p], rid) {
		e.refs[p] = append(e.refs[p], rid)
		slices.Sort(e.refs[p])
	}
}

// unref removes a resource from the ones pulling an image. An image no
// resource pulls is not the node's to hold any more: its key is dropped
// with its outcome, and the image left in the store, which is the
// operator's to prune.
func (t *Manager) unref(pull resource.Pull, p naming.Path, rid string) {
	e := t.keys[pull]
	if e == nil {
		return
	}
	e.refs[p] = slices.DeleteFunc(e.refs[p], func(s string) bool { return s == rid })
	if len(e.refs[p]) == 0 {
		delete(e.refs, p)
	}
	if len(e.refs) > 0 {
		return
	}
	delete(t.keys, pull)
	t.log.Debugf("%s: dropped, no resource pulls it any more", pull)
	if err := resource.RemovePrefetchOutcome(pull); err != nil {
		t.log.Warnf("%s: remove the outcome: %s", pull, err)
	}
}

// onImageRemoved has the settled keys of the store of an engine checked
// again, once the removals settled: whatever the image the event names, see
// the package comment. A key already to be checked keeps its time.
func (t *Manager) onImageRemoved(c *msgbus.ContainerImageRemoved) {
	engine := strings.TrimPrefix(c.Type, driverTypePrefix)
	at := t.now().Add(recheckDelay)
	n := 0
	for _, e := range t.keys {
		if e.pull.Engine != engine || e.pull.Store != c.Store || !e.dueAt.IsZero() {
			continue
		}
		e.dueAt = at
		n++
	}
	if n > 0 {
		t.log.Debugf("%s image removed (name=%q id=%q): check %d images again", engine, c.Name, c.ID, n)
	}
}

// nextWake is when the clock has something to do next, zero when it has
// nothing: a job runs, whose result is waited for, or no key is to be
// checked.
func (t *Manager) nextWake() time.Time {
	if t.running != nil {
		return time.Time{}
	}
	at := t.sweepAt
	for _, e := range t.keys {
		if !e.dueAt.IsZero() && (at.IsZero() || e.dueAt.Before(at)) {
			at = e.dueAt
		}
	}
	return at
}

// tick starts the job of the key due the longest, when no job runs.
func (t *Manager) tick() {
	if t.running != nil {
		return
	}
	now := t.now()
	if !t.sweepAt.IsZero() && !now.Before(t.sweepAt) {
		t.sweepAt = time.Time{}
		t.sweep()
	}
	var next *entry
	for _, e := range t.keys {
		if e.dueAt.IsZero() || e.dueAt.After(now) {
			continue
		}
		if next == nil || e.dueAt.Before(next.dueAt) || (e.dueAt.Equal(next.dueAt) && e.pull.Hash() < next.pull.Hash()) {
			next = e
		}
	}
	if next == nil {
		return
	}
	// What asks for a check from now on is another check to do, after
	// this one.
	next.dueAt = time.Time{}
	pull := next.pull
	t.running = &pull
	p, rid := next.target()
	t.start(job{pull: pull, path: p, rid: rid})
}

// target is the resource whose object runs the pull of the image: the first
// of the first object pulling it, by name, so a key is pulled by the same
// one from a try to the next.
func (e *entry) target() (naming.Path, string) {
	paths := slices.SortedFunc(maps.Keys(e.refs), func(a, b naming.Path) int {
		return strings.Compare(a.String(), b.String())
	})
	return paths[0], e.refs[paths[0]][0]
}

// do checks a key, and pulls its image when the store does not hold it. It
// is the part of a job run apart from the events, and touches no key.
func (t *Manager) do(ctx context.Context, j job) result {
	r := result{pull: j.pull}
	present, id, checked, err := t.check(ctx, j.pull)
	switch {
	case err != nil:
		r.err = err
		return r
	case checked && present:
		r.present = true
		r.id = id
		return r
	}
	// Missing, or in a store the daemon does not read: the command of
	// the object looks, and pulls. The outcome of a former prefetch of
	// the key stays in its file: the one read after the command is the
	// one of this command only when it is another.
	former, had := resource.ReadPrefetchOutcome(j.pull)
	if err := t.exec(ctx, j.path, j.rid); err != nil {
		r.err = err
		return r
	}
	outcome, ok := resource.ReadPrefetchOutcome(j.pull)
	if ok && had && outcome.UpdatedAt.Equal(former.UpdatedAt) && outcome.State == former.State {
		ok = false
	}
	if !ok {
		// The configuration changed between its publication and the
		// command, which prefetched another key: the next instance
		// config says which.
		r.err = fmt.Errorf("the prefetch of %s %s recorded no outcome for this image", j.path, j.rid)
		return r
	}
	r.outcome = &outcome
	return r
}

// onResult follows what a job did.
func (t *Manager) onResult(r result) {
	t.running = nil
	e := t.keys[r.pull]
	if e == nil {
		// Dropped while the job ran, which may have recorded its
		// outcome since.
		if err := resource.RemovePrefetchOutcome(r.pull); err != nil {
			t.log.Warnf("%s: remove the outcome: %s", r.pull, err)
		}
		return
	}
	former := e.state
	switch {
	case r.err != nil:
		// Nothing is known of the image: the state stays, and the
		// key is tried again as after a failed pull.
		e.attempts++
		t.retryLater(e)
		t.log.Warnf("%s: %s: next try in %s", r.pull, r.err, e.dueAt.Sub(t.now()).Round(time.Second))
		return
	case r.present:
		e.state = resource.PrefetchPresent
		e.imageID = r.id
		e.attempts = 0
		if former == resource.PrefetchFailed {
			// Pulled by a start, or by hand, since the prefetch
			// failed: the failure is not the news any more.
			if err := resource.RemovePrefetchOutcome(r.pull); err != nil {
				t.log.Warnf("%s: remove the outcome: %s", r.pull, err)
			}
		}
		t.log.Debugf("%s: present", r.pull)
	case r.outcome.State == resource.PrefetchFailed:
		e.state = resource.PrefetchFailed
		e.attempts++
		t.retryLater(e)
		t.log.Warnf("%s: %s: next try in %s", r.pull, r.outcome.Error, e.dueAt.Sub(t.now()).Round(time.Second))
	default:
		e.state = r.outcome.State
		e.imageID = r.outcome.ImageID
		e.attempts = 0
		if e.state == resource.PrefetchPulled {
			t.log.Infof("%s: fetched from %s", r.pull, r.outcome.Source)
		}
	}
	if (former == resource.PrefetchFailed) != (e.state == resource.PrefetchFailed) {
		msg := &msgbus.ImagePrefetchUpdated{Node: t.localhost, Pull: r.pull, State: e.state}
		if r.outcome != nil {
			msg.Error = r.outcome.Error
		}
		t.publisher.Pub(msg, t.labelLocalhost)
	}
}

// retryLater has a key that failed tried again after a delay doubled at each
// failure in a row, and drawn longer by up to a fifth, for the nodes failing
// together not to try together. A try asked sooner meanwhile, as by a
// configuration change, is kept.
func (t *Manager) retryLater(e *entry) {
	delay := maxRetryDelay
	if shift := e.attempts - 1; shift < 16 {
		delay = min(retryDelay<<max(shift, 0), maxRetryDelay)
	}
	at := t.now().Add(delay + t.jitter(delay/5))
	if e.dueAt.IsZero() || at.Before(e.dueAt) {
		e.dueAt = at
	}
}

// sweep removes the outcomes recorded for the keys the node does not have:
// the ones of resources removed, or whose prefetch was disabled, while the
// daemon did not run.
func (t *Manager) sweep() {
	for file, outcome := range resource.PrefetchOutcomes() {
		if _, ok := t.keys[outcome.Key]; ok {
			continue
		}
		if err := resource.RemovePrefetchOutcomeFile(file); err != nil {
			t.log.Warnf("remove the outcome %s: %s", file, err)
		} else {
			t.log.Debugf("removed the outcome %s, of no image the node is meant to hold", file)
		}
	}
}

// checkStore asks the store of the engine of a key whether it holds its
// image. It answers checked false for a store the daemon does not read: the
// one of a user, or of an engine no driver registered.
func checkStore(ctx context.Context, pull resource.Pull) (bool, string, bool, error) {
	if pull.Store != "" {
		return false, "", false, nil
	}
	store := resource.ImageStoreOf(pull.Engine)
	if store == nil {
		return false, "", false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	present, id, err := store.Has(ctx, pull)
	return present, id, true, err
}

// CmdArgs returns the om argv prefetching the image of a resource of an
// object.
//
// These words are a contract with the om command tree: core/om's
// TestPrefetchCmdArgsResolve keeps this function and the tree in sync.
func CmdArgs(p naming.Path, rid string) []string {
	return []string{p.String(), "instance", "prefetch", "--rid", rid}
}

// execCommand runs the prefetch command of an object, announced on the bus
// as the other commands the daemon runs. The command exits 0 on a pull that
// failed, which it records: an error is the command not doing its job.
func (t *Manager) execCommand(ctx context.Context, p naming.Path, rid string) error {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	const origin = "prefetch"
	execID := xsession.NewExecID()
	sessionID := xsession.NewSessionID()
	labels := []pubsub.Label{
		t.labelLocalhost,
		{"origin", origin},
		{"namespace", p.Namespace},
		{"path", p.String()},
	}
	// A failed om says why on the last "Error: " line of its stderr,
	// which no terminal shows here.
	var reason string
	cmd := command.New(
		command.WithName(t.om),
		command.WithArgs(CmdArgs(p, rid)),
		command.WithContext(ctx),
		command.WithLogger(t.log),
		command.WithEnv([]string{
			env.ActionOriginDaemonPrefetch.Var(),
			xsession.SessionID().ParentVar(),
			execID.Var(),
			sessionID.Var(),
		}),
		command.WithOnStderrLine(func(s string) {
			if after, ok := strings.CutPrefix(s, "Error: "); ok {
				reason = after
			}
		}),
	)
	startTime := t.now()
	t.publisher.Pub(&msgbus.Exec{
		Command:   cmd.String(),
		Node:      t.localhost,
		Origin:    origin,
		RID:       rid,
		ExecID:    execID,
		SessionID: sessionID,
		StartedAt: startTime,
	}, labels...)
	failed := func(err error, exitCode int) error {
		t.publisher.Pub(&msgbus.ExecFailed{
			Command:   cmd.String(),
			Duration:  t.now().Sub(startTime),
			ErrS:      err.Error(),
			ExitCode:  exitCode,
			Node:      t.localhost,
			Origin:    origin,
			ExecID:    execID,
			SessionID: sessionID,
		}, labels...)
		if reason != "" {
			return fmt.Errorf("%s: %w: %s", cmd, err, reason)
		}
		return fmt.Errorf("%s: %w", cmd, err)
	}
	if err := cmd.Start(); err != nil {
		// The start of this exec was announced, so its end has to be
		// too. There is no exit status to report: it never ran.
		return failed(err, -1)
	}
	pid := cmd.Cmd().Process.Pid
	proc.Register(proc.T{Pid: pid, ExecID: execID.String()})
	err := cmd.Wait()
	proc.Unregister(pid)
	if err != nil {
		return failed(err, cmd.NormalizedExitCode())
	}
	t.publisher.Pub(&msgbus.ExecSuccess{
		Command:   cmd.String(),
		Duration:  t.now().Sub(startTime),
		ExitCode:  cmd.NormalizedExitCode(),
		Node:      t.localhost,
		Origin:    origin,
		ExecID:    execID,
		SessionID: sessionID,
	}, labels...)
	return nil
}
