package hbucast

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
)

type (
	// tx holds a hb unicast transmitter
	tx struct {
		sync.WaitGroup
		ctx         context.Context
		id          string
		nodes       map[string]string
		addr        string
		port        string
		intf        string
		interval    time.Duration
		timeout     time.Duration
		localIP     net.IP
		lastNodeErr map[string]string

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
	for node := range t.nodes {
		t.cmdC <- hbctrl.CmdDelWatcher{
			HbID:     t.id,
			Nodename: node,
		}
	}
	t.Wait()
	t.log.Debugf("wait done")
	return nil
}

func (t *tx) streamPeerDesc(addr string) string {
	if len(t.localIP) > 0 {
		if t.intf != "" {
			return fmt.Sprintf("%s@%s → %s", t.localIP, t.intf, addr)
		} else {
			return fmt.Sprintf("%s → %s", t.localIP, addr)
		}
	} else {
		if t.intf != "" {
			return fmt.Sprintf("@%s → %s", t.intf, addr)
		} else {
			return fmt.Sprintf("→ %s", addr)
		}
	}
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
		t.log.Infof("starting: timeout %s, interval: %s", t.timeout, t.interval)
		for node, addr := range t.nodes {
			cmdC <- hbctrl.CmdAddWatcher{
				HbID:     t.id,
				Nodename: node,
				Ctx:      ctx,
				Timeout:  t.timeout,
				Desc:     t.streamPeerDesc(addr),
			}
		}
		started <- true
		var b []byte

		sendTicker := time.NewTicker(t.interval)
		defer sendTicker.Stop()

		localIPTicker := time.NewTicker(30 * time.Second)
		defer localIPTicker.Stop()

		updateLocalIP := func() {
			if localIP, err := t.defaultLocalIP(); err != nil {
				t.log.Errorf("%s", err)
			} else if !t.localIP.Equal(localIP) {
				t.log.Infof("set local ip to %s", localIP)
				t.localIP = localIP
			}
		}

		if localIP, err := t.defaultLocalIP(); err != nil {
			t.log.Errorf("%s", err)
		} else if localIP != nil {
			t.log.Infof("set local ip to %s", localIP)
			t.localIP = localIP
		} else {
			t.log.Infof("undetermined local ip")
		}

		var reason string
		for {
			select {
			case <-ctx.Done():
				t.log.Infof("stopped")
				return
			case b = <-msgC:
				reason = "send msg"
				sendTicker.Reset(t.interval)
			case <-sendTicker.C:
				reason = "send msg (interval)"
			case <-localIPTicker.C:
				updateLocalIP()
			}
			if len(b) == 0 {
				continue
			} else {
				t.log.Debugf(reason)
				for node, addr := range t.nodes {
					go t.send(node, addr, b)
				}
			}
		}
	}()
	<-started
	t.log.Infof("started")
	return nil
}

// defaultLocalIP returns the ip address of the local nodename, so rx on peer
// nodes see messages coming from a known cluster member.
func (t *tx) defaultLocalIP() (net.IP, error) {
	if t.addr != "" {
		return net.ParseIP(t.addr), nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(t.ctx, hostname.Hostname())
	if err != nil {
		return nil, fmt.Errorf("lookup sender addr: %s: %s", t.addr, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("lookup sender addr: %s: no address found ", t.addr)
	}
	return addrs[0].IP, nil
}

func (t *tx) send(node, addr string, b []byte) {
	localAddr := net.TCPAddr{
		IP:   t.localIP,
		Port: 0,
	}
	dialer := net.Dialer{
		Timeout:   t.timeout,
		LocalAddr: &localAddr,
	}
	send := func() error {
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return err
		}
		defer func() {
			_ = conn.Close()
		}()
		if err := conn.SetDeadline(time.Now().Add(t.timeout)); err != nil {
			return err
		}
		if n, err := conn.Write(b); err != nil {
			return err
		} else if n != len(b) {
			return err
		}
		return nil
	}

	clearDedupLog := func() {
		if _, ok := t.lastNodeErr[node]; !ok {
			return
		}
		t.log.Infof("end a send error period: %s", t.lastNodeErr[node])
		delete(t.lastNodeErr, node)
	}
	setDedupLog := func(err error) {
		lastErr, _ := t.lastNodeErr[node]
		newErr := err.Error()
		if newErr != lastErr {
			if lastErr != "" {
				t.log.Infof("end a send error period: %s", lastErr)
			} else {
				t.log.Warnf("begin a send error period: %s", newErr)
			}
			t.lastNodeErr[node] = newErr
		}
	}

	if err := send(); err != nil {
		setDedupLog(err)
		return
	}

	clearDedupLog()

	t.cmdC <- hbctrl.CmdSetPeerSuccess{
		Nodename: node,
		HbID:     t.id,
		Success:  true,
	}
}

func newTx(ctx context.Context, name string, nodes map[string]string, addr, port, intf string, timeout, interval time.Duration) *tx {
	id := name + ".tx"
	return &tx{
		ctx:         ctx,
		id:          id,
		nodes:       nodes,
		lastNodeErr: make(map[string]string),
		addr:        addr,
		port:        port,
		intf:        intf,
		interval:    interval,
		timeout:     timeout,
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbucast").
			Attr("hb_func", "tx").
			Attr("hb_name", name).
			Attr("hb_id", id).
			WithPrefix("daemon: hb: ucast: tx: " + name + ": "),
	}
}
