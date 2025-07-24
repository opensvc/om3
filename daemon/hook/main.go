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
		kind := t.config.Get(key.New(name, "events"))
		h := hook{
			sig: sig,
		}
		t.hooks[name] = h
		if !slices.Contains(AllowedEvents, kind) {
			t.log.Warnf("%s: event %s is not allowed in hooks", name, kind)
			continue
		}
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
		msg, err := msgbus.KindToT(kind)
		if err != nil {
			t.log.Warnf("%s: invalid event %s: %s", name, kind, err)
			continue
		}
		h.cancel = t.startHook(name, kind, msg, args)
		t.hooks[name] = h
	}
}

func (t *Manager) hookExec(ctx context.Context, event any, args []string) error {
	b, err := json.Marshal(event)
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

func (t *Manager) hookLoop(ctx context.Context, sub *pubsub.Subscription, name, kind string, args []string) {
	t.log.Infof("%s: listening for event %s", name, kind)
	defer t.log.Infof("%s: stop listening for event %s", name, kind)
	var running atomic.Bool

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-sub.C:
			if running.Load() {
				t.log.Warnf("%s: on %s command is too slow => skip exec", name, kind)
				continue
			}
			running.Store(true)
			go func() {
				defer running.Store(false)
				t.log.Infof("%s: on %s exec %s", name, kind, args)
				if err := t.hookExec(ctx, event, args); err != nil {
					t.log.Warnf("%s: %s", name, err)
				}
			}()
		}
	}
}

func (t *Manager) startHook(name, kind string, event any, args []string) func() {
	ctx, cancel := context.WithCancel(t.ctx)
	sub := pubsub.SubFromContext(ctx, "daemon.hook", t.subQS, pubsub.Timeout(time.Second))
	sub.AddFilter(event, t.labelLocalhost)
	sub.Start()

	go func() {
		t.hookLoop(ctx, sub, name, kind, args)
		sub.Stop()
	}()
	return cancel
}
