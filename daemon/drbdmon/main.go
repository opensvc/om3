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
// on every distribution the agent supports, down to el7. Where the
// requirements are not met, the monitor disables itself and says so once:
// drbdsetup not installed, or an event stream the installed drbd does not
// support or keeps failing. A node where the drbd kernel module is not loaded
// yet waits for it, as the first drbd resource brought up loads it. Without
// the monitor, a drbd state change is seen at the next scheduled status.
package drbdmon

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// Manager watches the drbd events and publishes a DrbdResourceUpdated
	// message for each drbd resource whose state changed.
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

		// drbdsetup is the path of the drbdsetup command, empty where drbd
		// is not installed.
		drbdsetup string

		// debounced holds the timers publishing the change of a drbd
		// resource, one per resource, so a state transition, which changes
		// several objects of a resource at once, is published once.
		debounced   map[string]*time.Timer
		debouncedMu sync.Mutex

		wg sync.WaitGroup
	}
)

const (
	// debounceDelay is how long the changes of a drbd resource settle
	// before they are published.
	debounceDelay = 500 * time.Millisecond

	// restartDelay is how long to wait before running the event stream
	// again when it ended.
	restartDelay = 5 * time.Second

	// moduleCheckInterval is how often a node where the drbd kernel module
	// is not loaded checks it again.
	moduleCheckInterval = 30 * time.Second

	// probeTimeout bounds the "drbdsetup events2 --now" probe, which prints
	// the current state and exits.
	probeTimeout = 10 * time.Second

	// quickFailureDuration is a run of the event stream short enough to
	// count as a failure to start it, and maxQuickFailures is how many in a
	// row disable the monitor.
	quickFailureDuration = 10 * time.Second
	maxQuickFailures     = 3
)

var (
	// procDRBD exists when the drbd kernel module is loaded.
	procDRBD = "/proc/drbd"
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
		debounced:      make(map[string]*time.Timer),
	}
}

// Start launches the drbdmon worker goroutine, where drbdsetup is installed
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)

	t.startSubscriptions()

	if p, err := exec.LookPath("drbdsetup"); err != nil {
		t.log.Infof("drbdsetup is not installed: no drbd event to watch")
	} else {
		t.drbdsetup = p
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			defer t.log.Infof("drbd monitor done")
			t.watch()
		}()
	}

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
	t.debouncedMu.Lock()
	for _, timer := range t.debounced {
		timer.Stop()
	}
	t.debouncedMu.Unlock()
	return nil
}

// startSubscriptions starts the pubsub subscriptions for control messages like AuditStart/AuditStop
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.drbdmon", t.subQS)

	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})

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
				}
			}
		}
	}()
}

// watch runs the drbd event stream, and runs it again when it ends, until
// the manager stops, or until the stream proves unusable.
func (t *Manager) watch() {
	if !t.waitModule() {
		return
	}
	if err := t.probe(); err != nil {
		t.log.Warnf("disabled: %s events2 is not supported by the installed drbd: %s: drbd state changes are seen at the scheduled status only", t.drbdsetup, err)
		return
	}
	quickFailures := 0
	for {
		t.log.Infof("watch %s events2", t.drbdsetup)
		begin := time.Now()
		err := t.readEvents()
		if t.ctx.Err() != nil {
			return
		}
		if err != nil {
			t.log.Warnf("%s events2: %s", t.drbdsetup, err)
		} else {
			t.log.Warnf("%s events2 ended", t.drbdsetup)
		}
		if time.Since(begin) < quickFailureDuration {
			quickFailures++
		} else {
			quickFailures = 0
		}
		if quickFailures >= maxQuickFailures {
			t.log.Warnf("disabled: %s events2 ended %d times in a row right after it started: drbd state changes are seen at the scheduled status only", t.drbdsetup, quickFailures)
			return
		}
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(restartDelay):
		}
	}
}

// waitModule waits for the drbd kernel module to be loaded, which the first
// drbd resource brought up on the node does. It returns false when the
// manager stops first.
func (t *Manager) waitModule() bool {
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
		case <-t.ctx.Done():
			return false
		case <-time.After(moduleCheckInterval):
		}
	}
}

// probe checks the installed drbd supports the event stream: "drbdsetup
// events2 --now" prints the current state and exits, with an error where
// the command or the kernel module does not know it.
func (t *Manager) probe() error {
	ctx, cancel := context.WithTimeout(t.ctx, probeTimeout)
	defer cancel()
	b, err := exec.CommandContext(ctx, t.drbdsetup, "events2", "--now").CombinedOutput()
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

// readEvents runs "drbdsetup events2" and notifies the drbd resources whose
// state changed, until the command ends.
func (t *Manager) readEvents() error {
	cmd := exec.CommandContext(t.ctx, t.drbdsetup, "events2")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if name, ok := parseEvent(scanner.Text()); ok {
			t.notify(name)
		}
	}
	errScan := scanner.Err()
	errWait := cmd.Wait()
	return errors.Join(errScan, errWait)
}

// notify publishes the change of the drbd resource name once its changes
// settle.
func (t *Manager) notify(name string) {
	t.debouncedMu.Lock()
	defer t.debouncedMu.Unlock()
	if timer, ok := t.debounced[name]; ok {
		timer.Reset(debounceDelay)
		return
	}
	t.debounced[name] = time.AfterFunc(debounceDelay, func() {
		t.debouncedMu.Lock()
		delete(t.debounced, name)
		t.debouncedMu.Unlock()
		t.log.Debugf("drbd resource %s changed", name)
		t.publisher.Pub(&msgbus.DrbdResourceUpdated{Node: t.localhost, Res: name}, t.labelLocalhost)
	})
}

// parseEvent returns the name of the drbd resource whose state an events2
// line reports a change of.
//
// The lines are "<event> <object> <key>:<value> ...". The "exists" lines
// describing the state when the stream starts, a change of fields the status
// does not read, and the objects it does not read are ignored.
func parseEvent(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", false
	}
	switch fields[0] {
	case "create", "change", "destroy":
	default:
		return "", false
	}
	if !objects[fields[1]] {
		return "", false
	}
	var name string
	changed := fields[0] != "change"
	for _, field := range fields[2:] {
		k, v, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		switch {
		case k == "name":
			name = v
		case stateKeys[k]:
			changed = true
		}
	}
	if name == "" || !changed {
		return "", false
	}
	return name, true
}
