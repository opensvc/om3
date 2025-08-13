package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/xmap"
)

type (
	Manager struct {
		// config is the node merged config
		config *xconfig.T

		ctx       context.Context
		cancel    context.CancelFunc
		log       *plog.Logger
		startedAt time.Time

		labelLocalhost pubsub.Label
		localhost      string

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		hooks hooks

		wg sync.WaitGroup
	}

	hook struct {
		sig    string
		cancel func()
	}

	hooks map[string]hook
)

var (
	AllowedEvents = []string{
		"ArbitratorError",
		"EnterOverloadPeriod",
		"ForgetPeer",
		"HeartbeatAlive",
		"HeartbeatStale",
		"HeartbeatMessageTypeUpdated",
		"InstanceMonitorAction",
		"LeaveOverloadPeriod",
		"NodeAlive",
		"NodeFrozen",
		"NodeSplitAction",
		"NodeStale",
	}
)

func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		log:       plog.NewDefaultLogger().Attr("pkg", "daemon/hook").WithPrefix("daemon: hook: "),
		localhost: localhost,

		labelLocalhost: pubsub.Label{"node", localhost},

		subQS: subQS,
		hooks: make(hooks),
	}
}

func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.update()
	t.startSubscriptions()
	t.startUpdateLoop()
	t.log.Infof("started")
	return nil
}

func (t *Manager) loadConfig() error {
	n, err := object.NewNode(object.WithVolatile(false))
	if err != nil {
		return err
	}
	t.config = n.MergedConfig()
	return nil
}

func (t *Manager) startUpdateLoop() {
	go func() {
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-t.sub.C:
				t.update()
			}
		}
	}()
}

func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.hook", t.subQS)
	sub.AddFilter(&msgbus.NodeConfigUpdated{}, t.labelLocalhost)
	sub.Start()
	t.sub = sub
}

func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) update() {
	if err := t.loadConfig(); err != nil {
		t.log.Warnf("%s", err)
		return
	}
	currentHookNames := xmap.Keys(t.hooks)
	var hooksToStop, scannedHooks []string
	hooksToStart := make(map[string]string)
	for _, name := range t.config.SectionStrings() {
		if !strings.HasPrefix(name, "hook#") {
			continue
		}
		scannedHooks = append(scannedHooks, name)
		currentHook, ok := t.hooks[name]
		if !ok {
			hooksToStart[name] = ""
			continue
		}
		sig := t.config.SectionSig(name)
		if sig != currentHook.sig {
			hooksToStop = append(hooksToStop, name)
			hooksToStart[name] = sig
		}

	}
	for _, name := range currentHookNames {
		if !slices.Contains(scannedHooks, name) {
			hooksToStop = append(hooksToStop, name)
		}
	}
	for _, name := range hooksToStop {
		if t.hooks[name].cancel != nil {
			t.hooks[name].cancel()
		}
		delete(t.hooks, name)
	}
	for name, sig := range hooksToStart {
		kinds := t.config.GetStrings(key.New(name, "events"))
		h := hook{
			sig: sig,
		}
		t.hooks[name] = h
		s := t.config.Get(key.New(name, "command"))
		args, err := command.CmdArgsFromString(s)
		if err != nil {
			t.log.Warnf("%s: failed to split command: %s", name, err)
			continue
		}
		if len(args) < 1 {
			t.log.Warnf("%s: empty command", name)
			continue
		}
		h.cancel = t.startHook(name, kinds, args)
		t.hooks[name] = h
	}
}

func (t *Manager) hookExec(ctx context.Context, ev *event.Event, args []string) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to json-encode event: %w", err)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = []string{
		"EVENT=" + string(b),
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create a new process group
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cmd: %w", err)
	}
	return cmd.Wait()
}

func (t *Manager) hookLoop(ctx context.Context, sub *pubsub.Subscription, name string, kinds, args []string) {
	t.log.Infof("%s: listening for events %s", name, kinds)
	defer t.log.Infof("%s: stop listening for events %s", name, kinds)
	var (
		running atomic.Bool
		j       uint64
	)

	for {
		select {
		case <-ctx.Done():
			return
		case i := <-sub.C:
			j += 1
			ev := event.ToEvent(i, j)
			if ev == nil {
				continue
			}
			if running.Load() {
				t.log.Warnf("%s: on %s command is too slow => skip exec", name, ev.Kind)
				continue
			}
			running.Store(true)
			go func() {
				defer running.Store(false)
				t.log.Infof("%s: on %s exec %s", name, ev.Kind, args)
				if err := t.hookExec(ctx, ev, args); err != nil {
					t.log.Warnf("%s: %s", name, err)
				}
			}()
		}
	}
}

func (t *Manager) startHook(name string, kinds []string, args []string) func() {
	ctx, cancel := context.WithCancel(t.ctx)
	sub := pubsub.SubFromContext(ctx, "daemon.hook", t.subQS, pubsub.Timeout(time.Second))
	added := 0
	for _, kind := range kinds {
		if !slices.Contains(AllowedEvents, kind) {
			t.log.Warnf("%s: event %s is not allowed in hooks", name, kind)
			continue
		}
		event, err := msgbus.KindToT(kind)
		if err != nil {
			t.log.Warnf("%s: invalid event %s: %s", name, kind, err)
			continue
		}
		sub.AddFilter(event)
		added += 1
	}
	if added == 0 {
		cancel()
		return nil
	}
	sub.Start()

	go func() {
		t.hookLoop(ctx, sub, name, kinds, args)
		sub.Stop()
	}()
	return cancel
}
