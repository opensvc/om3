package hbucast

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/plog"
)

type (
	// tx holds a hb unicast transmitter
	tx struct {
		sync.WaitGroup
		ctx      context.Context
		id       string
		nodes    []string
		port     string
		intf     string
		interval time.Duration
		timeout  time.Duration

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
)

// Id implements the Id function of Transmitter interface for tx
func (t *tx) Id() string {
	return t.id
}

// Stop implements the Stop function of Transmitter interface for tx
func (t *tx) Stop() error {
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

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("starting")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
			}
		}
		started <- true
		var b []byte
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		var reason string
		for {
			select {
			case <-ctx.Done():
				t.log.Infof("stopped")
				return
			case b = <-msgC:
				reason = "send msg"
				ticker.Reset(t.interval)
			case <-ticker.C:
				reason = "send msg (interval)"
			}
			if len(b) == 0 {
				continue
			} else {
				t.log.Debugf(reason)
				for _, node := range t.nodes {
					go t.send(node, b)
				}
			}
		}
	}()
	<-started
	t.log.Infof("started")
	return nil
}

func (t *tx) send(node string, b []byte) {
	conn, err := net.DialTimeout("tcp", node+":"+t.port, t.timeout)
	if err != nil {
		t.log.Debugf("dial timeout %s:%s: %s", node, t.port, err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()
	if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
		t.log.Errorf("set deadline %s:%s: %s", node, t.port, err)
		return
	}
	if n, err := conn.Write(b); err != nil {
		t.log.Debugf("write %s: %s", node, err)
		return
	} else if n != len(b) {
		t.log.Debugf("write %d instead of %d", n, len(b))
		return
	}
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: node,
		HbID:     t.id,
		Success:  true,
	}
}

func newTx(ctx context.Context, name string, nodes []string, port, intf string, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		port:     port,
		intf:     intf,
		interval: interval,
		timeout:  timeout,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbucast").
			Attr("hb_func", "tx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: ucast: tx: " + name + ": "),
	}
}
