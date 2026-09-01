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

		// failing is true while the relay is refusing or unreachable,
		// so that the transition is logged rather than every beat of an
		// outage.
		failing bool
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

func (t *tx) Ctx() context.Context {
	return t.ctx
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
		t.postLoop(ctx, msgC, sub.C, t.send)
	}()

	return <-errC
}

// postLoop posts the freshest message the daemon produced, at most once
// per interval and at least once per interval.
//
// The relay keeps one message per node, overwritten by each post, and a
// peer reads it once per its own interval. So the interval is what to
// post at: posting on every message the daemon produces, which is one
// per propagation tick when anything changes, overwrites mail nobody
// read yet, at a rate the relay has to carry for every cluster that
// uses it.
//
// The last message is kept after it was posted, because a post with the
// same payload refreshes the timestamp the peers read the liveness of
// this stream from.
func (t *tx) postLoop(ctx context.Context, msgC <-chan []byte, subC <-chan any, post func([]byte)) {
	var (
		latest     []byte
		lastSentAt time.Time
	)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	send := func() {
		if len(latest) == 0 {
			return
		}
		lastSentAt = time.Now()
		ticker.Reset(t.interval)
		t.log.Tracef("send msg")
		post(latest)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-msgC:
			latest = b
			if time.Since(lastSentAt) >= t.interval {
				// Nothing was posted for an interval, so this is not a
				// burst: a node that just started, or a cluster that
				// just changed after a quiet spell, reaches its peers
				// without waiting for the next tick.
				send()
			}
		case <-ticker.C:
			send()
		case ev := <-subC:
			t.onEvent(ev)
		}
	}
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
		t.logFailure("post to %s: %s", t.relay, err)
		return
	}

	defer drain(resp.Body, t.log)

	if resp.StatusCode != http.StatusOK {
		t.logFailure("post to %s: %s", t.relay, resp.Status)
		return
	}
	t.logRecovery()

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

// logFailure reports a beat the relay did not take.
//
// Only the first of a run is a warning: the beats are paced by the
// interval, so an outage would otherwise write one line per interval for
// as long as it lasts, and the state of the stream is already in the
// daemon status. The rest are traces, for an audit of the stream.
func (t *tx) logFailure(format string, a ...any) {
	if t.failing {
		t.log.Tracef(format, a...)
		return
	}
	t.failing = true
	t.log.Warnf(format, a...)
}

func (t *tx) logRecovery() {
	if !t.failing {
		return
	}
	t.failing = false
	t.log.Infof("post to %s: relaying again", t.relay)
}
