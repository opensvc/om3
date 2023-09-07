package hbmcast

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/omcrypto"
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
		laddr    *net.UDPAddr
		udpAddr  *net.UDPAddr
		interval time.Duration
		timeout  time.Duration

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
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC

	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Info().Msg("starting")
		defer t.log.Info().Msg("stopped")
		for _, node := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbId:     t.id,
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
				go t.send(b)
			}
		}
	}()
	<-started
	t.log.Info().Msg("started")
	return nil
}

func (t *tx) encryptMessage(b []byte) ([]byte, error) {
	cluster := ccfg.Get()
	msg := &omcrypto.Message{
		NodeName:    hostname.Hostname(),
		ClusterName: cluster.Name,
		Key:         cluster.Secret(),
		Data:        b,
	}
	return msg.Encrypt()
}

func (t *tx) send(b []byte) {
	//fmt.Println("xx >>>\n", hex.Dump(b))
	t.log.Debug().Msgf("send to udp %s", t.udpAddr)

	c, err := net.DialUDP("udp", t.laddr, t.udpAddr)
	if err != nil {
		t.log.Debug().Err(err).Msgf("dial udp %s", t.udpAddr)
		return
	}
	defer c.Close()
	msgID := uuid.New().String()
	msgLength := len(b)
	total := msgLength / MaxChunkSize
	if (msgLength % MaxChunkSize) != 0 {
		total += 1
	}
	if total > MaxFragments {
		// the message will not be sent by this heart beat.
		t.log.Error().Msgf("drop message for udp conn to %s: maximum fragment to create %d (message length %d)",
			t.udpAddr, total, msgLength)
		return
	}
	for i := 1; i <= total; i += 1 {
		f := fragment{
			MsgID: msgID,
			Index: i,
			Total: total,
		}
		if i == total {
			f.Chunk = b
		} else {
			f.Chunk = b[:MaxChunkSize]
			b = b[MaxChunkSize:]
		}
		dgram, err := json.Marshal(f)
		if err != nil {
			t.log.Debug().Err(err).Msgf("marshal frame")
			return
		}
		if _, err := c.Write(dgram); err != nil {
			t.log.Debug().Err(err).Msgf("write in udp conn to %s", t.udpAddr)
			return
		}
	}
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: node,
			HbId:     t.id,
			Success:  true,
		}
	}
}

func newTx(ctx context.Context, name string, nodes []string, laddr, udpAddr *net.UDPAddr, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	log := daemonlogctx.Logger(ctx).With().Str("id", id).Logger()
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		udpAddr:  udpAddr,
		laddr:    laddr,
		interval: interval,
		timeout:  timeout,
		log:      log,
	}
}
