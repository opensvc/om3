/*
Package hbucast implement a hb unicast driver

Example:

	msgC := make(chan *hbtype.Msg) // a global *hbtype.Msg chan for received Msg
	n := clusterhb.New()
	registeredDataC := make([]chan []byte, 0) // list send data chan where tx can send data
	for _, h := range n.Hbs() {
		h.Configure(ctx) // configure tx and rx
		if err := h.Rx().Start(data.Cmd(), msgC); err != nil {
			return err
		}
		localDataC := make(chan []byte) // []byte chan dedicated to this tx
		if err := h.Tx().Start(data.Cmd(), localDataC); err != nil {
			return err
		}
		registeredDataC = append(registeredDataC, localDataC)
	}
*/
package hbucast

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/opensvc/om3/core/hbcfg"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
)

type (
	T struct {
		hbcfg.T
	}
)

func New() hbcfg.Confer {
	t := &T{}
	var i interface{} = t
	return i.(hbcfg.Confer)
}

func init() {
	hbcfg.Register("unicast", New)
}

// Configure implements the Configure function of Confer interface for T
func (t *T) Configure(ctx context.Context) {
	log := plog.Logger{
		Logger: plog.PkgLogger(ctx, "daemon/hb/hbucast").With().Str("hb_name", t.Name()).Logger(),
		Prefix: "daemon: hb: ucast: " + t.Name() + ": ",
	}
	interval := t.GetDuration("interval", 5*time.Second)
	timeout := t.GetDuration("timeout", 15*time.Second)
	if timeout < 2*interval+1*time.Second {
		oldTimeout := timeout
		timeout = interval*2 + 1*time.Second
		log.Warnf("reajust timeout: %s => %s (<interval>*2+1s)", oldTimeout, timeout)
	}
	portI := t.GetInt("port")
	port := strconv.Itoa(portI)
	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Debugf("Configure timeout=%s interval=%s port=%s nodes=%s onodes=%s", timeout, interval,
		port, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetInterval(interval)
	t.SetTimeout(timeout)
	intf := t.GetString("intf")
	signature := fmt.Sprintf("type: hb.ucast, port: %s nodes: %s timeout: %s interval: %s intf: %s",
		port, nodes, timeout, interval, intf)
	t.SetSignature(signature)
	name := t.Name()
	tx := newTx(ctx, name, oNodes, port, intf, timeout, interval)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, "", port, intf, timeout)
	t.SetRx(rx)
}
