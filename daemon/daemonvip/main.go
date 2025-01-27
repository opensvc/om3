// Package daemonvip handle the system/svc/vip bootstrap and configuration
// updates.
package daemonvip

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/freeze"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		ctx       context.Context
		cancel    context.CancelFunc
		publisher pubsub.Publisher
		log       *plog.Logger

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		wg        sync.WaitGroup
		previous  cluster.Vip
		localhost string
	}
)

var (
	vipPath = naming.Path{Name: "vip", Namespace: "system", Kind: naming.KindSvc}
)

func New(subQS pubsub.QueueSizer) *T {
	localhost := hostname.Hostname()
	return &T{
		localhost: localhost,
		log: naming.
			LogWithPath(plog.NewDefaultLogger(), vipPath).
			Attr("pkg", "daemon/daemonvip").
			WithPrefix("daemon: vip: "),
		subQS: subQS,
	}
}

// Start launches the vip worker goroutine
func (t *T) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)

	t.wg.Add(1)
	go func() {
		defer func() {
			t.wg.Done()
			t.log.Infof("stopped")
		}()
		// Add delay to ensure icfg is started for vip
		// TODO: change to wait on discover initial discover loop is done ?
		time.Sleep(1000 * time.Millisecond)
		t.startSubscriptions()
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warnf("subscription stop: %s", err)
			}
		}()
		t.log.Infof("started")
		t.onClusterConfigUpdated(cluster.ConfigData.Get())
		t.worker()
	}()

	return nil
}

func (t *T) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *T) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.vip", t.subQS)
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", t.localhost})
	sub.Start()
	t.sub = sub
}

// worker watch for local ccfg updates
func (t *T) worker() {
	defer t.log.Debugf("done")
	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch c := i.(type) {
			case *msgbus.ClusterConfigUpdated:
				t.onClusterConfigUpdated(&c.Value)
			}
		}
	}
}

func (t *T) onClusterConfigUpdated(c *cluster.Config) {
	if c == nil || t.previous.Equal(&c.Vip) {
		return
	}
	if c.Vip.Devs == nil {
		if t.previous.Default != "" {
			t.log.Infof("will purge vip object from previous vip %s", t.previous)
			if err := t.purgeVip(); err != nil {
				t.log.Errorf("can't purge previous vip %s: %s", t.previous, err.Error())
			}
			t.previous = c.Vip
		}
		return
	}
	t.previous = c.Vip
	kv := map[string]string{
		"sync#i0.disable":          "true",
		"DEFAULT.nodes":            "*",
		"DEFAULT.orchestrate":      "ha",
		"DEFAULT.monitor_action":   "switch",
		"DEFAULT.monitor_schedule": "@1m",
		"DEFAULT.priority":         "0",
		"ip#0.ipname":              c.Vip.Addr,
		"ip#0.netmask":             c.Vip.Netmask,
		"ip#0.ipdev":               c.Vip.Dev,
		"ip#0.monitor":             "true",
		"ip#0.restart":             "1",
	}
	for n, dev := range c.Vip.Devs {
		kv["ip#0.ipdev@"+n] = dev
	}
	if len(instance.ConfigData.GetByPath(vipPath)) == 0 {
		t.log.Infof("will create vip instance from vip %s", c.Vip)
		err := t.createAndUnfreeze(kv)
		if err != nil {
			t.log.Errorf("create vip instance failed: %s", err.Error())
			return
		}
	} else {
		t.log.Infof("will update vip instance from vip %s", c.Vip)
		err := t.createOrUpdate(kv)
		if err != nil {
			t.log.Errorf("update vip instance failed: %s", err.Error())
			return
		}
	}
}

func (t *T) purgeVip() error {
	return t.orchestrate(instance.MonitorGlobalExpectPurged)
}

func (t *T) createAndUnfreeze(kv map[string]string) error {
	timeout := 2 * time.Second
	sub := pubsub.SubFromContext(t.ctx, "daemon.vip.createAndUnfreeze", pubsub.Timeout(timeout))
	waitCtx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, pubsub.Label{"path", vipPath.String()})
	sub.Start()
	defer func(sub *pubsub.Subscription) {
		err := sub.Stop()
		if err != nil {

		}
	}(sub)
	if err := freeze.Freeze(vipPath.FrozenFile()); err != nil {
		return fmt.Errorf("can't freeze instance: %w", err)
	}
	err := t.createOrUpdate(kv)
	if err != nil {
		return fmt.Errorf("create vip failed: %w", err)
	}
	// expectedInstances is defined from count of alive cluster nodes
	expectedInstances := len(node.ConfigData.GetAll())
	t.log.Infof("waiting for %s %d instances monitor...", vipPath, expectedInstances)
	imonIdles := make(map[string]struct{})
	for {
		select {
		case i := <-sub.C:
			switch m := i.(type) {
			case *msgbus.InstanceMonitorUpdated:
				if m.Value.State.Is(instance.MonitorStateIdle) {
					imonIdles[m.Node] = struct{}{}
				} else {
					delete(imonIdles, m.Node)
				}
				if len(imonIdles) >= expectedInstances {
					t.log.Infof("got enough vip instance monitors call orchestrate unfrozen")
					return t.orchestrate(instance.MonitorGlobalExpectUnfrozen)
				}
			}
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-waitCtx.Done():
			t.log.Warnf("waiting for instance monitor: %s", waitCtx.Err().Error())
			return waitCtx.Err()
		}
	}
}

func (t *T) createOrUpdate(kv map[string]string) error {
	o, err := object.NewConfigurer(vipPath)
	if err != nil {
		return err
	}
	rid := "ip#0"
	toSet := make([]keyop.T, 0)
	toUnset := make([]key.T, 0)
	if ipKw, err := o.Config().SectionMapStrict(rid); err == nil {
		for k := range ipKw {
			if k == "tags" {
				// never discard tag customization (ex: tags=noaction)
				continue
			}
			keyS := fmt.Sprintf("%s.%s", rid, k)
			keyT := key.T{Section: rid, Option: k}
			if _, ok := kv[keyS]; !ok {
				toUnset = append(toUnset, keyT)
			}
		}
	}

	for k, val := range kv {
		toSet = append(toSet, keyop.T{Key: key.Parse(k), Op: keyop.Set, Value: val})
	}
	t.log.Debugf("will set %#v, unset %#v", toSet, toUnset)
	return o.Update(t.ctx, nil, toUnset, toSet)
}

func (t *T) orchestrate(g instance.MonitorGlobalExpect) error {
	t.log.Infof("asking global expect: %s", g)
	timeout := time.Second
	ctx, cancel := context.WithTimeout(t.ctx, timeout)
	defer cancel()

	value := instance.MonitorUpdate{GlobalExpect: &g, CandidateOrchestrationID: uuid.New()}
	msg, setInstanceMonitorErr := msgbus.NewSetInstanceMonitorWithErr(ctx, vipPath, t.localhost, value)

	t.publisher.Pub(msg, []pubsub.Label{{"node", t.localhost}, pubsub.Label{"namespace", vipPath.Namespace}, {"path", vipPath.String()}}...)
	err := setInstanceMonitorErr.Receive()

	switch {
	case err == nil:
		t.log.Infof("global expect accepted: %s", g)
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("timeout waiting for global expect accepted")
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("context cancelled while waiting for global expect accepted")
	default:
		return fmt.Errorf("global expect refused: %s", err)
	}
}
