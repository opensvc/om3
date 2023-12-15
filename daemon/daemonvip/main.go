// Package daemonvip handle the system/svc/vip bootstrap and configuration
// updates.
package daemonvip

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
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
		bus       *pubsub.Bus
		log       *plog.Logger
		sub       *pubsub.Subscription
		wg        sync.WaitGroup
		previous  cluster.Vip
		localhost string
	}
)

var (
	vipPath = naming.Path{Name: "vip", Namespace: "system", Kind: naming.KindSvc}
)

func New() *T {
	localhost := hostname.Hostname()
	return &T{
		localhost: localhost,
		log: naming.
			LogWithPath(plog.NewDefaultLogger(), vipPath).
			Attr("pkg", "daemon/daemonvip").
			WithPrefix("daemon: vip: "),
	}
}

// Start launches the vip worker goroutine
func (t *T) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.bus = pubsub.BusFromContext(t.ctx)

	t.startSubscriptions()
	t.onClusterConfigUpdated(cluster.ConfigData.Get())
	t.wg.Add(1)
	go func() {
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warnf("subscription stop: %s", err)
			}
			t.wg.Done()
			defer t.log.Infof("stopped")
		}()
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
	sub := t.bus.Sub("vip")
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
		t.log.Infof("will create vip object from vip %s", c.Vip)
		err := t.createOrUpdate(kv)
		if err != nil {
			t.log.Errorf("create vip failed: %s", err.Error())
			return
		}
	} else {
		t.log.Infof("will update vip object from vip %s", c.Vip)
		err := t.createOrUpdate(kv)
		if err != nil {
			t.log.Errorf("update vip object failed: %s", err.Error())
			return
		}
	}
}

func (t *T) purgeVip() error {
	return nil
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
