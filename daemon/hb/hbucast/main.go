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
	"strconv"
	"time"

	"opensvc.com/opensvc/core/hbcfg"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
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
	log := daemonctx.Logger(ctx).With().Str("id", t.Name()+".tx").Logger()
	timeout := t.GetDuration("timeout", 5*time.Second)
	portI := t.GetInt("port")
	port := strconv.Itoa(portI)
	nodes := t.GetSlice("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetSlice(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Debug().Msgf("Configure %s, timeout=%s port=%s nodes=%s onodes=%s", t.Name(), timeout, port, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	intf := t.GetString("intf")
	name := t.Name()
	tx := newTx(ctx, name, oNodes, port, intf, timeout)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, "", port, intf, timeout)
	t.SetRx(rx)
}
