package hbrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/daemon/hb/hbdedup"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// rx holds a hb unicast receiver
	rx struct {
		sync.WaitGroup

		// decodeErrors logs the peers whose messages decrypt and do not
		// decode.
		decodeErrors hbctrl.DecodeErrors

		cfg

		ctx   context.Context
		nodes []string

		// lastAt is the time the relay stored the message of each peer,
		// as last read, written by the clock of the relay. A message
		// holding the same time again was not stored since, whatever the
		// clocks of the relay and this node say.
		lastAt map[string]time.Time

		name   string
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()

		crypto decryptWithNoder

		// dedup holds the frames the other hb links already delivered
		dedup *hbdedup.Cache

		// failing is true while the relay is refusing or unreachable,
		// so that the transition is logged rather than every beat of an
		// outage.
		failing bool
	}

	decryptWithNoder interface {
		DecryptWithNode(data []byte) ([]byte, string, error)
	}
)

// ID implements the ID function of the Receiver interface for rx
func (t *rx) ID() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
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

func (t *rx) streamPeerDesc() string {
	return fmt.Sprintf("← %s@%s", t.username, t.relay)
}

// Start implements the Start function of the Receiver interface for rx
func (t *rx) Start(cmdC chan<- any, msgC chan<- *hbtype.Msg) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel
	ticker := time.NewTicker(t.interval)

	for _, node := range t.nodes {
		cmdC <- hbctrl.CmdAddWatcher{
			HbID:     t.id,
			Nodename: node,
			Ctx:      ctx,
			Timeout:  t.timeout,
			Desc:     t.streamPeerDesc(),
		}
	}

	errC := make(chan error)
	t.Add(1)
	go func() {
		t.attachActiveAuditIfAny(ctx, "rx")
		sub := t.startSubscription(ctx)
		defer func() {
			ticker.Stop()
			_ = sub.Stop()
			t.Done()
			t.log.Infof("stopped")
		}()
		// ensure don't miss the first password update
		if err := t.refreshClient(); err != nil {
			t.log.Errorf("start: create client: %s", err)
			errC <- err
			return
		}
		t.log.Infof("started")
		errC <- nil
		crypto := hbcrypto.CryptoFromContext(ctx)
		t.dedup = hbdedup.CacheFromContext(ctx)
		for {
			select {
			case <-ctx.Done():
				t.cancel()
				return
			case <-ticker.C:
				if t.cli == nil {
					continue
				}
				t.crypto = crypto.Load()
				t.onTick()
			case ev := <-sub.C:
				t.onEvent(ev, "rx")
			}
		}
	}()

	return <-errC
}

func (t *rx) onTick() {
	for _, node := range t.nodes {
		t.recv(node)
	}
}

func (t *rx) recv(nodename string) {
	if t.cli == nil {
		return
	}
	clusterID := cluster.ConfigData.Get().ID

	params := api.GetRelayMessageParams{
		Nodename:  nodename,
		ClusterID: clusterID,
	}
	resp, err := t.cli.GetRelayMessageWithResponse(context.Background(), &params)
	if err != nil {
		t.logFailure("get %s from %s: %s", nodename, t.relay, err)
		return
	}

	defer drain(resp.HTTPResponse.Body, t.log)

	if resp.StatusCode() != http.StatusOK {
		t.logFailure("get %s from %s: %s", nodename, t.relay, resp.Status())
		return
	}
	t.logRecovery()
	if resp.JSON200 == nil {
		t.log.Tracef("recv: node %s data has no stored data", nodename)
		return
	}
	c := *resp.JSON200
	if c.UpdatedAt.IsZero() {
		t.log.Tracef("recv: node %s data has never been updated", nodename)
		return
	}
	if !t.isNewWrite(nodename, c.UpdatedAt) {
		t.log.Tracef("recv: node %s data not stored since last read, or too old on first read", nodename)
		return
	}
	frame := []byte(c.Msg)
	key := hbdedup.NewKey(frame)
	if msgNodename, ok := t.dedup.Seen(key); ok {
		// another hb link delivered this very frame: the peer is alive on
		// this one too, but the message is already in the daemon
		if nodename != msgNodename {
			t.log.Tracef("recv: node %s data was written by unexpected node %s", nodename, msgNodename)
			return
		}
		t.log.Tracef("recv: node %s, already delivered by another hb", nodename)
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: msgNodename,
			HbID:     t.id,
			Success:  true,
		}
		return
	}

	b, msgNodename, err := t.crypto.DecryptWithNode(frame)
	if err != nil {
		t.log.Tracef("recv: decrypting node %s: %s", nodename, err)
		return
	}

	if nodename != msgNodename {
		t.log.Tracef("recv: node %s data was written by unexpected node %s: %s", nodename, msgNodename, err)
		return
	}

	// The message decrypted, from the node that wrote it: the node is
	// alive, whether or not this agent can read what it says.
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: nodename,
		HbID:     t.id,
		Success:  true,
	}
	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.decodeErrors.Failed(t.log, nodename, err)
		return
	}
	t.decodeErrors.Succeeded(t.log, nodename)
	t.log.Tracef("recv: node %s", nodename)
	t.msgC <- &msg
	t.dedup.Delivered(key, msg.Nodename)
}

func newRx(ctx context.Context, name string, nodes []string, cfg cfg) *rx {
	id := name + ".rx"
	cfg.id = id
	cfg.log = plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbrelay").
		Attr("hb_func", "rx").
		Attr("hb_name", name).
		Attr("hb_id", id).
		WithPrefix("daemon: hb: relay: rx: " + name + ": ")

	return &rx{
		ctx:    ctx,
		nodes:  nodes,
		cfg:    cfg,
		lastAt: make(map[string]time.Time),
	}
}

// isNewWrite says whether the relay stored a message of a peer since the last
// read, and records the store time read.
//
// A peer is alive when the relay stored a message of it since the last read.
// The age of the store, measured against the clock of this node, says nothing
// once there is a last read to compare with: the relay stamps it with its own
// clock, and a relay whose clock runs ahead would keep a dead peer looking
// alive for as long as it runs ahead. The first read has nothing to compare
// with, so a message too old to be the one of a live peer is left for the
// next one.
func (t *rx) isNewWrite(nodename string, storedAt time.Time) bool {
	last, seen := t.lastAt[nodename]
	t.lastAt[nodename] = storedAt
	if seen {
		return !storedAt.Equal(last)
	}
	return time.Since(storedAt) <= t.timeout
}

// logFailure reports a beat the relay did not answer. Only the first of
// a run is a warning, as for the transmitter.
func (t *rx) logFailure(format string, a ...any) {
	if t.failing {
		t.log.Tracef(format, a...)
		return
	}
	t.failing = true
	t.log.Warnf(format, a...)
}

func (t *rx) logRecovery() {
	if !t.failing {
		return
	}
	t.failing = false
	t.log.Infof("get from %s: reading again", t.relay)
}
