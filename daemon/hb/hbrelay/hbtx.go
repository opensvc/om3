package hbrelay

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	tx struct {
		sync.WaitGroup

		cfg

		ctx   context.Context
		nodes []string

		name string

		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
)

// ID implements the ID function of Transmitter interface for tx
func (t *tx) ID() string {
	return t.id
}

// Stop implements the Stop function of Transmitter interface for tx
func (t *tx) Stop() error {
	t.log.Tracef("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Tracef("wait done")
	return nil
}

func (t *tx) streamPeerDesc() string {
	return fmt.Sprintf("→ %s@%s", t.username, t.relay)
}

// Start implements the Start function of Transmitter interface for tx
func (t *tx) Start(cmdC chan<- interface{}, msgC <-chan []byte) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	errC := make(chan error)
	t.Add(1)
	go func() {
		t.attachActiveAuditIfAny(ctx)
		sub := t.startSubscription(ctx)
		defer func() { _ = sub.Stop() }()
		if err := t.refreshClient(); err != nil {
			t.log.Errorf("start: create client: %s", err)
			errC <- err
			return
		}
		t.log.Infof("started")
		errC <- nil
		defer func() {
			t.Done()
			t.log.Infof("stopped")
		}()
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
				Desc:     t.streamPeerDesc(),
			}
		}
		var b []byte
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		var reason string
		for {
			select {
			case <-ctx.Done():
				return
			case b = <-msgC:
				reason = "send msg"
				ticker.Reset(t.interval)
			case <-ticker.C:
				reason = "send msg (interval)"
			case ev := <-sub.C:
				t.onEvent(ev)
			}
			if len(b) == 0 {
				continue
			} else {
				t.log.Tracef(reason)
				t.send(b)
			}
		}
	}()

	return <-errC
}

func (t *tx) send(b []byte) {
	if t.cli == nil {
		return
	}

	clusterConfig := cluster.ConfigData.Get()
	params := api.PostRelayMessage{
		Nodename:    hostname.Hostname(),
		ClusterID:   clusterConfig.ID,
		ClusterName: clusterConfig.Name,
		Msg:         string(b),
	}
	resp, err := t.cli.PostRelayMessage(context.Background(), params)
	if err != nil {
		t.log.Tracef("send: PostRelayMessage: %s", err)
		return
	}

	defer drain(resp.Body, t.log)

	if resp.StatusCode != http.StatusOK {
		t.log.Tracef("send: unexpected PostRelayMessage status: %s", resp.Status)
		return
	}

	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: node,
			HbID:     t.id,
			Success:  true,
		}
	}
}

func newTx(ctx context.Context, name string, nodes []string, cfg cfg) *tx {
	id := name + ".tx"
	cfg.id = id
	cfg.log = plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbrelay").
		Attr("hb_func", "tx").
		Attr("hb_name", name).
		Attr("hb_id", id).
		WithPrefix("daemon: hb: relay: tx: " + name + ": ")

	return &tx{
		ctx:   ctx,
		nodes: nodes,
		cfg:   cfg,
	}
}
