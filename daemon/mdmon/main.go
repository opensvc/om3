// Package mdmon watches the state changes of the md arrays of the node, and
// publishes them as pubsub events, for the instances holding an md resource
// to evaluate their status again.
//
// An md array changes state with no action of the node to tell: a resync or
// a recovery ends, seconds or hours after the action that started it, a
// device fails or is added back. Without the monitor, the change is seen at
// the next scheduled status, with the instance said resyncing or degraded
// meanwhile.
//
// It waits for the md events the kernel signals on /proc/mdstat, as mntmon
// waits for the mount events on /proc/self/mountinfo and as "mdadm
// --monitor" does, while the node holds an md resource (see package
// evwatch). The kernel does not tell which array changed: the arrays whose
// lines of /proc/mdstat changed are, and the instance holding each refreshes
// its status, by the kernel name of its array its md resource tells.
//
// A change is published debounceDelay after it is read, and only when a
// local instance holds the array then: an array an instance just started is
// named in its status by the end of its action, which the delay leaves time
// for. An event changing no line, as an array resynced whole between two
// reads, names no array, for the instances whose md resource status still
// tells a transient state, as resyncing, to refresh.
//
// Where the md module is not loaded, the monitor waits for it, as the first
// array created loads it.
//
// It is not mdadm's own mdmon, the monitor of the arrays of external
// metadata.
package mdmon

import (
	"context"
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
	// Manager watches the md events of the node and publishes a
	// MdArrayUpdated message when the node holds an md resource.
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

		// counter counts the md resources of the local instances, by the
		// kernel name of their array, as "md127", an empty one for a
		// resource whose array is not assembled.
		counter *evwatch.Counter

		// runner runs the event source while the node holds an md
		// resource.
		runner *evwatch.Runner

		// debouncer publishes the events of an array once, as an array
		// started then resyncing signals twice, keyed by its kernel
		// name, "" for every array.
		debouncer *evwatch.Debouncer

		wg sync.WaitGroup
	}
)

const (
	// debounceDelay is how long after an md event its change is
	// published: the events of an array settle, and an array an instance
	// just started is named in its status by then.
	debounceDelay = 2 * time.Second

	// mdType is the driver type of the md resources, and devInfoKey the
	// status info naming the kernel name of their array.
	mdType     = "disk.md"
	devInfoKey = "dev"
)

// NewManager creates a new md monitor manager
func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/mdmon").WithPrefix("daemon: mdmon: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
		counter:        evwatch.NewCounter(),
	}
}

// Start starts the md monitor, which waits for the md events while the node
// holds an md resource.
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)
	t.debouncer = evwatch.NewDebouncer(debounceDelay, t.publish)
	t.runner = evwatch.NewRunner(newSource(t.log), evwatch.DefaultPolicy, t.log, t.resync, t.onEvent)
	t.runner.Start(t.ctx)

	t.startSubscriptions()

	// The statuses published before the subscription, as on a component
	// started late, are counted too.
	for p, st := range instance.StatusData.GetByNode(t.localhost) {
		if st != nil {
			t.counter.Set(p.String(), evwatch.KeysOf(*st, devInfoKey, mdType))
		}
	}
	t.runner.SetDemand(t.counter.Len())

	t.log.Infof("started")
	return nil
}

// Stop stops the md monitor manager
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
// tell the md resources the node holds.
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.mdmon", t.subQS)

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
					t.log.HandleAuditStart(c.Q, c.Subsystems, "mdmon")
				case *msgbus.AuditStop:
					t.log.HandleAuditStop(c.Q, c.Subsystems, "mdmon")
				case *msgbus.InstanceStatusUpdated:
					if c.Node == t.localhost {
						t.counter.Set(c.Path.String(), evwatch.KeysOf(c.Value, devInfoKey, mdType))
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

// onEvent notifies the change of the array name, "" for no array named.
// Whether a local instance holds it is told when it is published.
func (t *Manager) onEvent(name string) {
	t.debouncer.Notify(name)
}

// resync notifies a change when the source starts: what changed while it did
// not run was told by no event.
func (t *Manager) resync() {
	if t.counter.Len() > 0 {
		t.debouncer.Notify("")
	}
}

// publish publishes the change of the md array name when a local instance
// holds it, or of no array named, "", when a local instance holds an md
// resource. The others, as an array no object names, interest no instance.
func (t *Manager) publish(name string) {
	if !t.isTaken(name) {
		t.log.Debugf("md array %q changed, held by no local instance", name)
		return
	}
	t.log.Debugf("md array %q changed", name)
	t.publisher.Pub(&msgbus.MdArrayUpdated{Node: t.localhost, Name: name}, t.labelLocalhost)
}

// isTaken says a local instance holds the array name, or any md resource for
// no array named.
func (t *Manager) isTaken(name string) bool {
	if name == "" {
		return t.counter.Len() > 0
	}
	return t.counter.Has(name)
}
