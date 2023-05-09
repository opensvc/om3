package hbrelay

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
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
		log    zerolog.Logger
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
	t.log.Debug().Msg("cancelling")
	t.cancel()
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbId:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Debug().Msg("wait done")
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
		t.log.Info().Msg("started")
		defer t.log.Info().Msg("stopped")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
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
				t.log.Debug().Msg(reason)
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
		t.log.Debug().Err(err).Msg("send: new client")
		return
	}

	cluster := ccfg.Get()
	params := api.PostRelayMessage{
		Nodename:    hostname.Hostname(),
		ClusterId:   cluster.ID,
		ClusterName: cluster.Name,
		Msg:         string(b),
	}
	resp, err := cli.PostRelayMessage(context.Background(), params)
	if err != nil {
		t.log.Debug().Err(err).Msg("send: PostRelayMessage")
		return
	} else if resp.StatusCode != http.StatusOK {
		t.log.Debug().Msgf("send: unexpected PostRelayMessage status: %s", resp.Status)
		return
	}

	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: node,
			HbId:     t.id,
			Success:  true,
		}
	}
}

func newTx(ctx context.Context, name string, nodes []string, relay, username, password string, insecure bool, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
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
		log:      log,
	}
}
