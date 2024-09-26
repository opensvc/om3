package hbdisk

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/omcrypto"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
)

type (
	// rx holds an hb unicast receiver
	rx struct {
		sync.WaitGroup
		base     base
		ctx      context.Context
		id       string
		nodes    []string
		timeout  time.Duration
		interval time.Duration
		last     time.Time

		name   string
		log    *plog.Logger
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()

		encryptDecrypter *omcrypto.Factory
	}
)

// ID implements the ID function of the Receiver interface for rx
func (t *rx) ID() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
	t.log.Debugf("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Debugf("wait done")
	return nil
}

// Start implements the Start function of the Receiver interface for rx
func (t *rx) Start(cmdC chan<- any, msgC chan<- *hbtype.Msg) error {
	if err := t.base.device.open(); err != nil {
		return err
	}
	if err := t.base.LoadPeerConfig(t.nodes); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel

	clusterConfig := cluster.ConfigData.Get()
	t.encryptDecrypter = &omcrypto.Factory{
		NodeName:    hostname.Hostname(),
		ClusterName: clusterConfig.Name,
		Key:         clusterConfig.Secret(),
	}

	for _, node := range t.nodes {
		cmdC <- hbctrl.CmdAddWatcher{
			HbID:     t.id,
			Nodename: node,
			Ctx:      ctx,
			Timeout:  t.timeout,
		}
	}

	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("started")
		defer t.log.Infof("stopped")
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.onTick()
			case <-ctx.Done():
				t.cancel()
				return
			}
		}
	}()
	return nil
}

func (t *rx) onTick() {
	for _, node := range t.nodes {
		t.recv(node)
	}
}

func (t *rx) recv(nodename string) {
	meta, err := t.base.GetPeer(nodename)
	if err != nil {
		t.log.Debugf("recv: failed to allocate a slot for node %s: %s", nodename, err)
		return
	}
	c, err := t.base.ReadDataSlot(meta.Slot) // TODO read timeout?
	if err != nil {
		t.log.Debugf("recv: reading node %s data slot %d: %s", nodename, meta.Slot, err)
		return
	}
	if c.Updated.IsZero() {
		t.log.Debugf("recv: node %s data slot %d has never been updated", nodename, meta.Slot)
		return
	}
	if !t.last.IsZero() && c.Updated == t.last {
		t.log.Debugf("recv: node %s data slot %d has not change since last read", nodename, meta.Slot)
		return
	}
	elapsed := time.Now().Sub(c.Updated)
	if elapsed > t.timeout {
		t.log.Debugf("recv: node %s data slot %d has not been updated for %s", nodename, meta.Slot, elapsed)
		return
	}
	b, msgNodename, err := t.encryptDecrypter.DecryptWithNode(c.Msg)
	if err != nil {
		t.log.Debugf("recv: decrypting node %s data slot %d: %s", nodename, meta.Slot, err)
		return
	}

	if nodename != msgNodename {
		t.log.Debugf("recv: node %s data slot %d was written by unexpected node %s", nodename, meta.Slot, msgNodename)
		return
	}

	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.log.Warnf("can't unmarshal msg from %s: %s", nodename, err)
		return
	}
	t.log.Debugf("recv: node %s", nodename)
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbID:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
	t.last = c.Updated
}

func newRx(ctx context.Context, name string, nodes []string, dev string, timeout, interval time.Duration) *rx {
	id := name + ".rx"
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbdisk").
		Attr("hb_func", "rx").
		Attr("hb_name", name).
		Attr("hb_id", id).
		WithPrefix("daemon: hb: disk: rx: " + name + ": ")

	return &rx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		timeout:  timeout,
		interval: interval,
		log:      log,
		base: base{
			log: log,
			device: device{
				path: dev,
			},
		},
	}
}
