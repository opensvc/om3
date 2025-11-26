package hbrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/plog"
)

type (
	// rx holds a hb unicast receiver
	rx struct {
		sync.WaitGroup
		ctx      context.Context
		id       string
		nodes    []string
		relay    string
		username string
		password string
		insecure bool
		timeout  time.Duration
		interval time.Duration
		lastAt   time.Time

		name   string
		log    *plog.Logger
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()

		crypto decryptWithNoder
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

	t.Add(1)
	go func() {
		defer t.Done()
		defer ticker.Stop()
		t.log.Infof("started")
		defer t.log.Infof("stopped")
		crypto := hbcrypto.CryptoFromContext(ctx)
		for {
			select {
			case <-ctx.Done():
				t.cancel()
				return
			case <-ticker.C:
				t.crypto = crypto.Load()
				t.onTick()
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
	clusterID := cluster.ConfigData.Get().ID
	cli, err := client.New(
		client.WithURL(t.relay),
		client.WithUsername(t.username),
		client.WithPassword(t.password),
		client.WithInsecureSkipVerify(t.insecure),
	)
	if err != nil {
		t.log.Errorf("recv: node %s new client: %s", nodename, err)
		return
	}

	params := api.GetRelayMessageParams{
		Nodename:  nodename,
		ClusterID: clusterID,
	}
	resp, err := cli.GetRelayMessageWithResponse(context.Background(), &params)
	if err != nil {
		t.log.Tracef("recv: node %s do request: %s", nodename, err)
		return
	}

	defer drain(resp.HTTPResponse.Body, t.log)

	if resp.StatusCode() != http.StatusOK {
		t.log.Tracef("unexpected get relay message %s status %s", nodename, resp.Status())
		return
	}
	if resp.JSON200 == nil {
		t.log.Tracef("recv: node %s data has no stored data", nodename)
		return
	}
	c := *resp.JSON200
	if c.UpdatedAt.IsZero() {
		t.log.Tracef("recv: node %s data has never been updated", nodename)
		return
	}
	if !t.lastAt.IsZero() && c.UpdatedAt == t.lastAt {
		t.log.Tracef("recv: node %s data has not change since last read", nodename)
		return
	}
	elapsed := time.Now().Sub(c.UpdatedAt)
	if elapsed > t.timeout {
		t.log.Tracef("recv: node %s data has not been updated for %s", nodename, elapsed)
		return
	}
	b, msgNodename, err := t.crypto.DecryptWithNode([]byte(c.Msg))
	if err != nil {
		t.log.Tracef("recv: decrypting node %s: %s", nodename, err)
		return
	}

	if nodename != msgNodename {
		t.log.Tracef("recv: node %s data was written by unexpected node %s: %s", nodename, msgNodename, err)
		return
	}

	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.log.Warnf("can't unmarshal msg from %s: %s", nodename, err)
		return
	}
	t.log.Tracef("recv: node %s", nodename)
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbID:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
	t.lastAt = c.UpdatedAt
}

func newRx(ctx context.Context, name string, nodes []string, relay, username, password string, insecure bool, timeout, interval time.Duration) *rx {
	id := name + ".rx"
	return &rx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		relay:    relay,
		username: username,
		password: password,
		insecure: insecure,
		timeout:  timeout,
		interval: interval,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbrelay").
			Attr("hb_func", "rx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: relay: rx: " + name + ": "),
	}
}
