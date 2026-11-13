// Package ctrmon watches the state changes of the podman and docker
// containers, and publishes them as pubsub events, for the instances holding
// a container resource to evaluate their status again.
//
// A container changes state with no action of the node to tell: its process
// exits, the kernel kills it for memory, an admin stops or removes it, its
// health check fails. Without the monitor, the change is seen at the next
// scheduled status: within the monitor schedule for a monitored resource, the
// status schedule otherwise, with the instance said up meanwhile.
//
// It reads the event stream of each engine, "podman events" and "docker
// events", while the node holds a container resource of that engine: a node
// holding none has no container event to wait for, and a "docker events"
// keeps a connection to dockerd open (see package evwatch). A stream ending,
// as the engine restarting ends it, runs again after a delay, and the
// containers held are refreshed once it runs. Where an engine is not
// installed, its stream is disabled with a log line, until no container
// resource of it is left.
//
// The containers a podman of a user runs, rootless, are in the storage of
// that user, which the event stream of root does not see: they are seen at
// the scheduled status, as before.
package ctrmon

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
	// Manager watches the container events of each engine, and publishes a
	// ContainerUpdated message for each container of the local instances
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

		engines []*engine

		wg sync.WaitGroup
	}

	// engine is the watch of the containers of one engine.
	engine struct {
		// driverType is the type of the container resources the engine
		// runs, as "container.podman".
		driverType string

		// counter counts the container resources of the engine, by the
		// name of their container, which the events name.
		counter *evwatch.Counter

		runner    *evwatch.Runner
		debouncer *evwatch.Debouncer
	}

	// source is the event stream of an engine.
	source struct {
		log *plog.Logger

		// command is the engine command, as "podman", and args the ones
		// printing its container events, one json object per line.
		command string
		args    []string

		// parse returns the name of the container an event line says
		// changed state.
		parse func(string) (string, bool)

		// path is the path of the command, found at the check.
		path string
	}
)

const (
	// debounceDelay is how long the events of a container settle before
	// its change is published.
	debounceDelay = 500 * time.Millisecond

	// readyDelay is how long the stream runs before it is taken as
	// listening: an engine prints nothing once it is.
	readyDelay = 2 * time.Second

	// helpTimeout bounds the "<engine> events --help" check.
	helpTimeout = 10 * time.Second

	// nameInfoKey is the status info naming the container of a resource.
	nameInfoKey = "name"
)

// NewManager creates a new container monitor manager
func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/ctrmon").WithPrefix("daemon: ctrmon: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
	}
}

// sources returns the event source of each engine, with the driver type of
// the container resources it runs.
func (t *Manager) sources() map[string]*source {
	return map[string]*source{
		"container.podman": {
			log:     t.log,
			command: "podman",
			args:    []string{"events", "--format", "json", "--filter", "type=container"},
			parse:   parsePodmanEvent,
		},
		"container.docker": {
			log:     t.log,
			command: "docker",
			args:    []string{"events", "--format", "{{json .}}", "--filter", "type=container"},
			parse:   parseDockerEvent,
		},
	}
}

// Start starts the container monitor, which runs the event stream of each
// engine while the node holds a container resource of it.
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)
	for driverType, src := range t.sources() {
		e := &engine{driverType: driverType, counter: evwatch.NewCounter()}
		e.debouncer = evwatch.NewDebouncer(debounceDelay, func(name string) { t.publish(e.driverType, name) })
		e.runner = evwatch.NewRunner(src, evwatch.DefaultPolicy, t.log, func() { e.resync() }, func(name string) { e.onEvent(name) })
		e.runner.Start(t.ctx)
		t.engines = append(t.engines, e)
	}

	t.startSubscriptions()

	// The statuses published before the subscription, as on a component
	// started late, are counted too.
	for p, st := range instance.StatusData.GetByNode(t.localhost) {
		if st != nil {
			t.setStatus(p.String(), *st)
		}
	}

	t.log.Infof("started")
	return nil
}

// Stop stops the container monitor manager
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
	for _, e := range t.engines {
		e.runner.Stop()
		e.debouncer.Stop()
	}
	return nil
}

// setStatus counts the container resources of each engine the instance at
// path holds.
func (t *Manager) setStatus(path string, st instance.Status) {
	for _, e := range t.engines {
		e.counter.Set(path, evwatch.KeysOf(st, nameInfoKey, e.driverType))
		e.runner.SetDemand(e.counter.Len())
	}
}

// unsetStatus counts no container resource for the instance at path any
// more.
func (t *Manager) unsetStatus(path string) {
	for _, e := range t.engines {
		e.counter.Unset(path)
		e.runner.SetDemand(e.counter.Len())
	}
}

// startSubscriptions starts the pubsub subscriptions: the control messages
// like AuditStart/AuditStop, and the status of the local instances, which
// tell the container resources the node holds.
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.ctrmon", t.subQS)

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
					t.log.HandleAuditStart(c.Q, c.Subsystems, "ctrmon")
				case *msgbus.AuditStop:
					t.log.HandleAuditStop(c.Q, c.Subsystems, "ctrmon")
				case *msgbus.InstanceStatusUpdated:
					if c.Node == t.localhost {
						t.setStatus(c.Path.String(), c.Value)
					}
				case *msgbus.InstanceStatusDeleted:
					if c.Node == t.localhost {
						t.unsetStatus(c.Path.String())
					}
				}
			}
		}
	}()
}

// onEvent notifies the change of a container the local instances hold. The
// others, as a container no object names, interest no instance.
func (e *engine) onEvent(name string) {
	if e.counter.Has(name) {
		e.debouncer.Notify(name)
	}
}

// resync notifies every container of the engine the local instances hold,
// when its stream starts: what changed while it did not run was told by no
// event.
func (e *engine) resync() {
	for _, name := range e.counter.Keys() {
		e.debouncer.Notify(name)
	}
}

// publish publishes the change of the container name of the resources of
// driverType.
func (t *Manager) publish(driverType, name string) {
	t.log.Debugf("%s container %s changed", driverType, name)
	t.publisher.Pub(&msgbus.ContainerUpdated{Node: t.localhost, Type: driverType, Name: name}, t.labelLocalhost)
}

// Name implements evwatch.Source.
func (t *source) Name() string {
	return t.command + " events"
}

// Check implements evwatch.Source: the engine is installed, and prints its
// events as json.
func (t *source) Check(ctx context.Context) error {
	p, err := exec.LookPath(t.command)
	if err != nil {
		return fmt.Errorf("%s is not installed: no %s event to watch: %w", t.command, t.command, evwatch.ErrUnsupported)
	}
	ctx, cancel := context.WithTimeout(ctx, helpTimeout)
	defer cancel()
	b, err := exec.CommandContext(ctx, p, "events", "--help").CombinedOutput()
	if err != nil || !strings.Contains(string(b), "--format") {
		return fmt.Errorf("the installed %s prints no event stream: its container changes are seen at the scheduled status only: %w", t.command, evwatch.ErrUnsupported)
	}
	t.path = p
	return nil
}

// Run implements evwatch.Source: it reads the container events of the
// engine, ready once the stream ran readyDelay.
func (t *source) Run(ctx context.Context, ready func(), emit func(string)) error {
	t.log.Infof("watch %s events", t.path)
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
	stderr, err := evwatch.StreamLines(ctx, nil, func(line string) {
		if name, ok := t.parse(line); ok {
			emit(name)
		}
	}, t.path, t.args...)
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
