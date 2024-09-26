package hbrelay

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
)

type (
	tx struct {
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

		name   string
		log    *plog.Logger
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
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC
	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("started")
		defer t.log.Infof("stopped")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
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
			}
			if len(b) == 0 {
				continue
			} else {
				t.log.Debugf(reason)
				t.send(b)
			}
		}
	}()
	return nil
}

func (t *tx) send(b []byte) {
	cli, err := client.New(
		client.WithURL(t.relay),
		client.WithUsername(t.username),
		client.WithPassword(t.password),
		client.WithInsecureSkipVerify(t.insecure),
	)
	if err != nil {
		t.log.Debugf("send: new client: %s", err)
		return
	}

	clusterConfig := clusterdump.ConfigData.Get()
	params := api.PostRelayMessage{
		Nodename:    hostname.Hostname(),
		ClusterID:   clusterConfig.ID,
		ClusterName: clusterConfig.Name,
		Msg:         string(b),
	}
	resp, err := cli.PostRelayMessage(context.Background(), params)
	if err != nil {
		t.log.Debugf("send: PostRelayMessage: %s", err)
		return
	} else if resp.StatusCode != http.StatusOK {
		t.log.Debugf("send: unexpected PostRelayMessage status: %s", resp.Status)
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

func newTx(ctx context.Context, name string, nodes []string, relay, username, password string, insecure bool, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	return &tx{
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
			Attr("hb_func", "tx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: relay: tx: " + name + ": "),
	}
}
