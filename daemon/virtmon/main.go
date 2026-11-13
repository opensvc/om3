// Package virtmon watches the state changes of the libvirt domains, and
// publishes them as pubsub events, for the instances holding a kvm resource
// to evaluate their status again.
//
// A domain changes state with no action of the node to tell: its guest shuts
// it down, its qemu dies, an admin destroys or migrates it by hand. Without
// the monitor, the change is seen at the next scheduled status: within the
// monitor schedule for a monitored resource, the status schedule otherwise,
// with the instance said up meanwhile.
//
// It reads the event stream "virsh -r event --event lifecycle --loop"
// prints, on a read-only connection, while the node holds a kvm resource: a
// node holding none has no domain event to wait for, and the connection would
// keep a socket-activated libvirtd running (see package evwatch). A stream
// ending, as libvirtd restarting ends it, runs again after a delay, and the
// domains held are refreshed once it runs. Where virsh is not installed, or
// too old to print the events, the monitor disables itself and says so once,
// until no kvm resource is left.
package virtmon

import (
	"context"
	"errors"
	"fmt"
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
	// Manager watches the libvirt domain events and publishes a
	// VirtDomainUpdated message for each domain of the local instances
	// whose state changed.
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

		// counter counts the kvm resources of the local instances, by the
		// name of their domain, which the events name.
		counter *evwatch.Counter

		// runner runs the event stream while the node holds a kvm
		// resource.
		runner *evwatch.Runner

		// debouncer publishes the change of a domain once, as a start
		// prints two events.
		debouncer *evwatch.Debouncer

		wg sync.WaitGroup
	}

	// source is the "virsh event" stream.
	source struct {
		log *plog.Logger

		// virsh is the path of the virsh command, found at the check.
		virsh string
	}
)

const (
	// debounceDelay is how long the events of a domain settle before its
	// change is published.
	debounceDelay = 500 * time.Millisecond

	// readyDelay is how long the stream runs before it is taken as
	// listening: virsh prints nothing once its callback is registered.
	readyDelay = 2 * time.Second

	// helpTimeout bounds the "virsh help event" check.
	helpTimeout = 10 * time.Second

	// kvmType is the driver type of the kvm resources, and nameInfoKey the
	// status info naming their domain.
	kvmType     = "container.kvm"
	nameInfoKey = "name"
)

// env is the environment of virsh, which translates the events it prints.
var env = []string{"LC_ALL=C"}

// NewManager creates a new libvirt monitor manager
func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/virtmon").WithPrefix("daemon: virtmon: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
		counter:        evwatch.NewCounter(),
	}
}

// Start starts the libvirt monitor, which runs the event stream while the
// node holds a kvm resource.
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)
	t.debouncer = evwatch.NewDebouncer(debounceDelay, t.publish)
	t.runner = evwatch.NewRunner(&source{log: t.log}, evwatch.DefaultPolicy, t.log, t.resync, t.onEvent)
	t.runner.Start(t.ctx)

	t.startSubscriptions()

	// The statuses published before the subscription, as on a component
	// started late, are counted too.
	for p, st := range instance.StatusData.GetByNode(t.localhost) {
		if st != nil {
			t.counter.Set(p.String(), evwatch.KeysOf(*st, nameInfoKey, kvmType))
		}
	}
	t.runner.SetDemand(t.counter.Len())

	t.log.Infof("started")
	return nil
}

// Stop stops the libvirt monitor manager
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
// tell the kvm resources the node holds.
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.virtmon", t.subQS)

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
					t.log.HandleAuditStart(c.Q, c.Subsystems, "virtmon")
				case *msgbus.AuditStop:
					t.log.HandleAuditStop(c.Q, c.Subsystems, "virtmon")
				case *msgbus.InstanceStatusUpdated:
					if c.Node == t.localhost {
						t.counter.Set(c.Path.String(), evwatch.KeysOf(c.Value, nameInfoKey, kvmType))
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

// onEvent notifies the change of a domain the local instances hold. The
// others, as a domain no object names, interest no instance.
func (t *Manager) onEvent(name string) {
	if t.counter.Has(name) {
		t.debouncer.Notify(name)
	}
}

// resync notifies every domain the local instances hold, when the stream
// starts: what changed while it did not run was told by no event.
func (t *Manager) resync() {
	for _, name := range t.counter.Keys() {
		t.debouncer.Notify(name)
	}
}

// publish publishes the change of the domain name.
func (t *Manager) publish(name string) {
	t.log.Debugf("libvirt domain %s changed", name)
	t.publisher.Pub(&msgbus.VirtDomainUpdated{Node: t.localhost, Name: name}, t.labelLocalhost)
}

// Name implements evwatch.Source.
func (t *source) Name() string {
	return "virsh event"
}

// Check implements evwatch.Source: virsh is installed, and knows the event
// stream, which "virsh help event" tells with no connection to libvirtd.
func (t *source) Check(ctx context.Context) error {
	p, err := exec.LookPath("virsh")
	if err != nil {
		return fmt.Errorf("virsh is not installed: no libvirt event to watch: %w", evwatch.ErrUnsupported)
	}
	ctx, cancel := context.WithTimeout(ctx, helpTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p, "help", "event")
	cmd.Env = append(cmd.Environ(), env...)
	b, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(b), "--loop") {
		return fmt.Errorf("the installed virsh prints no event stream: the libvirt domain changes are seen at the scheduled status only: %w", evwatch.ErrUnsupported)
	}
	t.virsh = p
	return nil
}

// Run implements evwatch.Source: it reads the lifecycle events of the
// domains, on a read-only connection, ready once the stream ran readyDelay.
func (t *source) Run(ctx context.Context, ready func(), emit func(string)) error {
	t.log.Infof("watch %s event", t.virsh)
	ended := make(chan struct{})
	defer close(ended)
	go func() {
		select {
		case <-ctx.Done():
		case <-ended:
		case <-time.After(readyDelay):
			ready()
		}
	}()
	stderr, err := evwatch.StreamLines(ctx, env, func(line string) {
		if ev, ok := parseEvent(line); ok {
			t.log.Debugf("libvirt domain %s: %s %s", ev.Domain, ev.Event, ev.Detail)
			emit(ev.Domain)
		}
	}, t.virsh, "-r", "event", "--event", "lifecycle", "--loop")
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
