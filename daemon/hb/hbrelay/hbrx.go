package hbrelay

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/client"
	reqjsonrpc "github.com/opensvc/om3/core/client/requester/jsonrpc"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
)

type (
	// rx holds an hb unicast receiver
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
		last     time.Time

		name   string
		log    zerolog.Logger
		cmdC   chan<- any
		msgC   chan<- *hbtype.Msg
		cancel func()
	}
)

// Id implements the Id function of the Receiver interface for rx
func (t *rx) Id() string {
	return t.id
}

// Stop implements the Stop function of the Receiver interface for rx
func (t *rx) Stop() error {
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

// Start implements the Start function of the Receiver interface for rx
func (t *rx) Start(cmdC chan<- any, msgC chan<- *hbtype.Msg) error {
	ctx, cancel := context.WithCancel(t.ctx)
	t.cmdC = cmdC
	t.msgC = msgC
	t.cancel = cancel
	ticker := time.NewTicker(t.interval)

	for _, node := range t.nodes {
		cmdC <- hbctrl.CmdAddWatcher{
			HbId:     t.id,
			Nodename: node,
			Ctx:      ctx,
			Timeout:  t.timeout,
		}
	}

	t.Add(1)
	go func() {
		defer t.Done()
		defer ticker.Stop()
		t.log.Info().Msg("started")
		defer t.log.Info().Msg("stopped")
		for {
			select {
			case <-ctx.Done():
				t.cancel()
				return
			case <-ticker.C:
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
	cluster := ccfg.Get()
	cli, err := client.New(
		client.WithURL(t.relay),
		client.WithUsername(t.username),
		client.WithPassword(t.password),
		client.WithInsecureSkipVerify(t.insecure),
	)
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: node %s new client", nodename)
		return
	}

	params := api.GetRelayMessageParams{
		Nodename:  &nodename,
		ClusterId: &cluster.ID,
	}
	resp, err := cli.GetRelayMessageWithResponse(context.Background(), &params)
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: node %s do request", nodename)
		return
	}
	if resp.JSON200 == nil {
		t.log.Debug().Msgf("recv: node %s data has no stored data", nodename)
		return
	}
	messages := resp.JSON200
	if len(messages.Messages) == 0 {
		t.log.Debug().Msgf("recv: node %s data has no stored data", nodename)
		return
	}
	c := messages.Messages[0]
	if c.Updated.IsZero() {
		t.log.Debug().Msgf("recv: node %s data has never been updated", nodename)
		return
	}
	if !t.last.IsZero() && c.Updated == t.last {
		t.log.Debug().Msgf("recv: node %s data has not change since last read", nodename)
		return
	}
	elapsed := time.Now().Sub(c.Updated)
	if elapsed > t.timeout {
		t.log.Debug().Msgf("recv: node %s data has not been updated for %s", nodename, elapsed)
		return
	}
	encMsg := reqjsonrpc.NewMessage([]byte(c.Msg))
	b, msgNodename, err := encMsg.DecryptWithNode()
	if err != nil {
		t.log.Debug().Err(err).Msgf("recv: decrypting node %s", nodename)
		return
	}

	if nodename != msgNodename {
		t.log.Debug().Err(err).Msgf("recv: node %s data was written by unexpected node %s", nodename, msgNodename)
		return
	}

	msg := hbtype.Msg{}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.log.Warn().Err(err).Msgf("can't unmarshal msg from %s", nodename)
		return
	}
	t.log.Debug().Msgf("recv: node %s", nodename)
	//t.log.Debug().Msgf("recv: node %s unmarshaled %#v", nodename, msg)
	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: msg.Nodename,
		HbId:     t.id,
		Success:  true,
	}
	t.msgC <- &msg
	t.last = c.Updated
}

func newRx(ctx context.Context, name string, nodes []string, relay, username, password string, insecure bool, timeout, interval time.Duration) *rx {
	id := name + ".rx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
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
		log:      log,
	}
}
