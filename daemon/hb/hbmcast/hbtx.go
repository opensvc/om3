package hbmcast

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/omcrypto"
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
		laddr    *net.UDPAddr
		udpAddr  *net.UDPAddr
		interval time.Duration
		timeout  time.Duration

		name   string
		log    *plog.Logger
		cmdC   chan<- interface{}
		msgC   chan<- *hbtype.Msg
		cancel func()

		encryptDecrypter *omcrypto.Factory
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
	started := make(chan bool)
	ctx, cancel := context.WithCancel(t.ctx)
	t.cancel = cancel
	t.cmdC = cmdC

	clusterConfig := cluster.ConfigData.Get()
	t.encryptDecrypter = &omcrypto.Factory{
		NodeName:    hostname.Hostname(),
		ClusterName: clusterConfig.Name,
		Key:         clusterConfig.Secret(),
	}

	t.Add(1)
	go func() {
		defer t.Done()
		t.log.Infof("starting")
		defer t.log.Infof("stopped")
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
				go t.send(b)
			}
		}
	}()
	<-started
	t.log.Infof("started")
	return nil
}

func (t *tx) encryptMessage(b []byte) ([]byte, error) {
	return t.encryptDecrypter.Encrypt(b)
}

func (t *tx) send(b []byte) {
	//fmt.Println("xx >>>\n", hex.Dump(b))
	t.log.Debugf("send to udp %s", t.udpAddr)

	c, err := net.DialUDP("udp", t.laddr, t.udpAddr)
	if err != nil {
		t.log.Debugf("dial udp %s: %s", t.udpAddr, err)
		return
	}
	defer c.Close()
	msgID := uuid.New().String()
	msgLength := len(b)
	total := msgLength / MaxChunkSize
	if (msgLength % MaxChunkSize) != 0 {
		total++
	}
	if total > MaxFragments {
		// the message will not be sent by this heart beat.
		t.log.Errorf("drop message for udp conn to %s: maximum fragment to create %d (message length %d)",
			t.udpAddr, total, msgLength)
		return
	}
	for i := 1; i <= total; i++ {
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
			t.log.Debugf("marshal frame: %s", err)
			return
		}
		if _, err := c.Write(dgram); err != nil {
			t.log.Debugf("write in udp conn to %s: %s", t.udpAddr, err)
			return
		}
	}
	for _, node := range t.nodes {
		t.cmdC <- hbctrl.CmdSetPeerSuccess{
			Nodename: node,
			HbID:     t.id,
			Success:  true,
		}
	}
}

func newTx(ctx context.Context, name string, nodes []string, laddr, udpAddr *net.UDPAddr, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	return &tx{
		ctx:      ctx,
		id:       id,
		nodes:    nodes,
		udpAddr:  udpAddr,
		laddr:    laddr,
		interval: interval,
		timeout:  timeout,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbmcast").
			Attr("hb_func", "tx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: mcast: tx: " + name + ": "),
	}
}
