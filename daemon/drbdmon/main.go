// Package drbdmon watches the state changes of the drbd resources, and
// publishes them as pubsub events, for the instances holding a drbd resource
// to evaluate their status again.
//
// A drbd resource changes state with no action of the node to tell: its peers
// connect once it is up, a resync ends when it ends, a peer taking its disk
// down leaves a connection lost. The status read at the end of an action, or
// on a peer event, sees the state of that moment, and whatever follows stays
// unseen until the next scheduled status. Guessing when to read it again does
// not work for a resync, whose length depends on how much data it moves.
//
// It reads the event stream "drbdsetup events2" prints, which drbd 9 offers
// on every distribution the agent supports, down to el7, while the node holds
// a drbd resource: a node holding none has no drbd event to wait for (see
// package evwatch). Where the requirements are not met, the monitor disables
// itself and says so once, until no drbd resource is left: drbdsetup not
// installed, or an event stream the installed drbd does not support or keeps
// failing. A node where the drbd kernel module is not loaded yet waits for
// it, as the first drbd resource brought up loads it. Without the monitor, a
// drbd state change is seen at the next scheduled status.
package drbdmon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/daemon/evwatch"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// Manager watches the drbd events and publishes a DrbdResourceUpdated
	// message for each drbd resource of the local instances whose state
	// changed.
	Manager struct {
		drainDuration time.Duration

		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger

		publisher pubsub.Publisher
		sub       *pubsub.Subscription
		subQS     pubsub.QueueSizer

		localhost      string
		labelLocalhost pubsub.Label

		// counter counts the drbd resources of the local instances, by
		// the name of their drbd resource, which the events name.
		counter *evwatch.Counter

		// runner runs the event stream while the node holds a drbd
		// resource.
		runner *evwatch.Runner

		// debouncer publishes the change of a drbd resource once, as a
		// state transition changes several objects of a resource at once.
		debouncer *evwatch.Debouncer

		wg sync.WaitGroup
	}

	// source is the "drbdsetup events2" stream.
	source struct {
		log *plog.Logger

		// drbdsetup is the path of the drbdsetup command, found at the
		// check.
		drbdsetup string
	}
)

const (
	// debounceDelay is how long the changes of a drbd resource settle
	// before they are published.
	debounceDelay = 500 * time.Millisecond

	// moduleCheckInterval is how often a node where the drbd kernel module
	// is not loaded checks it again.
	moduleCheckInterval = 30 * time.Second

	// probeTimeout bounds the "drbdsetup events2 --now" probe, which prints
	// the current state and exits.
	probeTimeout = 10 * time.Second

	// probeWaitDelay bounds the wait for the output of the probe once it
	// exited or timed out.
	probeWaitDelay = 3 * time.Second

	// drbdType is the driver type of the drbd resources, and resInfoKey
	// the status info naming their drbd resource.
	drbdType   = "disk.drbd"
	resInfoKey = "res"
)

var (
	// procDRBD exists when the drbd kernel module is loaded.
	procDRBD = "/proc/drbd"

	// policy runs the stream as the other watchers do, but for the quick
	// failures: a stream ending 3 times in a row right after it started is
	// not supported by the installed drbd, and is not tried again until no
	// drbd resource is left.
	policy = func() evwatch.Policy {
		p := evwatch.DefaultPolicy
		p.MaxQuickFailures = 3
		return p
	}()
)

var (
	// objects are the drbd objects whose state the status of a drbd
	// resource reads. The paths and the helper calls tell nothing of it.
	objects = map[string]bool{
		"resource":    true,
		"connection":  true,
		"device":      true,
		"peer-device": true,
	}

	// stateKeys are the fields whose change changes the status of a drbd
	// resource. A change of the promotion score alone, which comes with
	// every connection change, does not.
	stateKeys = map[string]bool{
		"role":        true,
		"disk":        true,
		"peer-disk":   true,
		"connection":  true,
		"replication": true,
	}
)

// NewManager creates a new drbd monitor manager
func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/drbdmon").WithPrefix("daemon: drbdmon: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
		counter:        evwatch.NewCounter(),
	}
}

// Start starts the drbd monitor, which runs the event stream while the node
// holds a drbd resource.
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)
	t.debouncer = evwatch.NewDebouncer(debounceDelay, t.publish)
	t.runner = evwatch.NewRunner(&source{log: t.log}, policy, t.log, t.resync, t.onEvent)
	t.runner.Start(t.ctx)

	t.startSubscriptions()

	// The statuses published before the subscription, as on a component
	// started late, are counted too.
	for p, st := range instance.StatusData.GetByNode(t.localhost) {
		if st != nil {
			t.counter.Set(p.String(), evwatch.KeysOf(*st, resInfoKey, drbdType))
		}
	}
	t.runner.SetDemand(t.counter.Len())

	t.log.Infof("started")
	return nil
}

// Stop stops the drbdmon manager
func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	if t.sub != nil {
		if err := t.sub.Stop(); err != nil {
			t.log.Warnf("subscription stop: %s", err)
		}
	}
	t.wg.Wait()
	if t.runner != nil {
		t.runner.Stop()
	}
	if t.debouncer != nil {
		t.debouncer.Stop()
	}
	return nil
}

// startSubscriptions starts the pubsub subscriptions: the control messages
// like AuditStart/AuditStop, and the status of the local instances, which
// tell the drbd resources the node holds.
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.drbdmon", t.subQS)

	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	sub.AddFilter(&msgbus.InstanceStatusUpdated{}, t.labelLocalhost)
	sub.AddFilter(&msgbus.InstanceStatusDeleted{}, t.labelLocalhost)

	sub.Start()
	t.sub = sub

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			case ev := <-sub.C:
				switch c := ev.(type) {
				case *msgbus.AuditStart:
					t.log.HandleAuditStart(c.Q, c.Subsystems, "drbdmon")
				case *msgbus.AuditStop:
					t.log.HandleAuditStop(c.Q, c.Subsystems, "drbdmon")
				case *msgbus.InstanceStatusUpdated:
					if c.Node == t.localhost {
						t.counter.Set(c.Path.String(), evwatch.KeysOf(c.Value, resInfoKey, drbdType))
						t.runner.SetDemand(t.counter.Len())
					}
				case *msgbus.InstanceStatusDeleted:
					if c.Node == t.localhost {
						t.counter.Unset(c.Path.String())
						t.runner.SetDemand(t.counter.Len())
					}
				}
			}
		}
	}()
}

// onEvent notifies the change of a drbd resource the local instances hold.
// The others, as one made by hand, interest no instance.
func (t *Manager) onEvent(name string) {
	if t.counter.Has(name) {
		t.debouncer.Notify(name)
	}
}

// resync notifies every drbd resource the local instances hold, when the
// stream starts: what changed before, as the first connect of a drbd
// resource the provision just made, or while the stream was down, was told
// by no event.
func (t *Manager) resync() {
	for _, name := range t.counter.Keys() {
		t.debouncer.Notify(name)
	}
}

// publish publishes the change of the drbd resource name.
func (t *Manager) publish(name string) {
	t.log.Debugf("drbd resource %s changed", name)
	t.publisher.Pub(&msgbus.DrbdResourceUpdated{Node: t.localhost, Res: name}, t.labelLocalhost)
}

// Name implements evwatch.Source.
func (t *source) Name() string {
	return "drbdsetup events2"
}

// Check implements evwatch.Source: drbdsetup is installed, and, where the
// drbd kernel module is loaded, the installed drbd supports the event
// stream. Where it is not loaded yet, the stream checks it once it is.
func (t *source) Check(ctx context.Context) error {
	p, err := exec.LookPath("drbdsetup")
	if err != nil {
		return fmt.Errorf("drbdsetup is not installed: no drbd event to watch: %w", evwatch.ErrUnsupported)
	}
	t.drbdsetup = p
	if _, err := os.Stat(procDRBD); err != nil {
		return nil
	}
	if err := t.probe(ctx); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			// A probe timed out, or that could not run, says nothing of
			// the installed drbd: it is tried again.
			return err
		}
		return fmt.Errorf("events2 is not supported by the installed drbd: %s: the drbd state changes are seen at the scheduled status only: %w", err, evwatch.ErrUnsupported)
	}
	return nil
}

// Run implements evwatch.Source: it waits for the drbd kernel module, then
// reads the event stream, ready once its initial dump of the current state
// ends.
func (t *source) Run(ctx context.Context, ready func(), emit func(string)) error {
	if !t.waitModule(ctx) {
		return ctx.Err()
	}
	t.log.Infof("watch %s events2", t.drbdsetup)
	states := newEventStates()
	stderr, err := evwatch.StreamLines(ctx, nil, func(line string) {
		if line == "exists -" {
			ready()
			return
		}
		if name, ok := states.parse(line); ok {
			emit(name)
		}
	}, t.drbdsetup, "events2")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if stderr != "" {
		err = errors.Join(err, errors.New(stderr))
	}
	if err == nil {
		err = fmt.Errorf("ended")
	}
	return err
}

// waitModule waits for the drbd kernel module to be loaded, which the first
// drbd resource brought up on the node does. It returns false when the
// context is done first.
func (t *source) waitModule(ctx context.Context) bool {
	logged := false
	for {
		if _, err := os.Stat(procDRBD); err == nil {
			return true
		}
		if !logged {
			t.log.Infof("the drbd kernel module is not loaded: wait for it")
			logged = true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(moduleCheckInterval):
		}
	}
}

// probe checks the installed drbd supports the event stream: "drbdsetup
// events2 --now" prints the current state and exits, with an error where
// the command or the kernel module does not know it.
func (t *source) probe(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, t.drbdsetup, "events2", "--now")
	// A child holding the output open does not hold the probe longer.
	cmd.WaitDelay = probeWaitDelay
	b, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		// Killed for its timeout: an exit error that says nothing of
		// the installed drbd.
		return fmt.Errorf("events2 --now: %w", ctx.Err())
	}
	if err != nil {
		if msg := strings.TrimSpace(string(b)); msg != "" {
			return fmt.Errorf("%w: %s", err, firstLine(msg))
		}
		return err
	}
	return nil
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// eventStates are the values of the state keys of each drbd object, as the
// events2 lines of one stream say them: a "change" line repeats every field
// of its object, the ones that did not change too, as the disk state of a
// device whose open count changed. Each object is known by its resource
// name, and its peer node id and volume when it has them.
type eventStates map[string]map[string]string

func newEventStates() eventStates {
	return make(eventStates)
}

// parse returns the name of the drbd resource whose state an events2 line
// reports a change of.
//
// The lines are "<event> <object> <key>:<value> ...". The "exists" lines,
// describing the state when the stream starts, are recorded, and so is
// every line after, so a "change" line counts only when a state key takes
// another value: the open count of a device, which a process opening it as
// lvs scanning the block devices changes, the promotion score, the path and
// helper lines, and the objects the status does not read, count for
// nothing. A "create" or a "destroy" line always counts.
func (t eventStates) parse(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", false
	}
	event, object := fields[0], fields[1]
	switch event {
	case "exists", "create", "change", "destroy":
	default:
		return "", false
	}
	if !objects[object] {
		return "", false
	}
	var name, peer, volume string
	values := make(map[string]string)
	for _, field := range fields[2:] {
		k, v, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		switch {
		case k == "name":
			name = v
		case k == "peer-node-id":
			peer = v
		case k == "volume":
			volume = v
		case stateKeys[k]:
			values[k] = v
		}
	}
	if name == "" {
		return "", false
	}
	key := object + " " + name + " " + peer + " " + volume
	switch event {
	case "exists", "create":
		t[key] = values
		return name, event == "create"
	case "destroy":
		delete(t, key)
		if object == "resource" {
			// Its connections, devices and peer devices go with it.
			for k := range t {
				if strings.Fields(k)[1] == name {
					delete(t, k)
				}
			}
		}
		return name, true
	}
	known, ok := t[key]
	if !ok {
		known = make(map[string]string)
		t[key] = known
	}
	changed := false
	for k, v := range values {
		if previous, ok := known[k]; !ok || previous != v {
			changed = true
			known[k] = v
		}
	}
	return name, changed
}
