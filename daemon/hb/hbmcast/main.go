/*
Package hbmcast implement a hb multicast driver
*/
package hbmcast

import (
	"context"
	"fmt"
	"net"
	"time"

	"opensvc.com/opensvc/core/hbcfg"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

type (
	T struct {
		hbcfg.T
	}

	fragment struct {
		MsgID string `json:"mid"`
		Chunk []byte `json:"c"`
		Index int    `json:"i"`
		Total int    `json:"n"`
	}
)

var (
	MaxMessages     = 100
	MaxFragments    = 1000
	MaxData         = 1000
	MaxDatagramSize = 8192
)

func New() hbcfg.Confer {
	t := &T{}
	var i interface{} = t
	return i.(hbcfg.Confer)
}

func init() {
	hbcfg.Register("multicast", New)
}

// Configure implements the Configure function of Confer interface for T
func (t *T) Configure(ctx context.Context) {
	log := daemonlogctx.Logger(ctx).With().Str("id", t.Name()+".tx").Logger()
	timeout := t.GetDuration("timeout", 5*time.Second)
	port := t.GetInt("port")
	addr := t.GetString("addr")
	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Debug().Msgf("configure %s, timeout=%s port=%d nodes=%s onodes=%s", t.Name(), timeout, port, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	intf := t.GetString("intf")
	name := t.Name()

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		log.Error().Err(err).Msgf("configure %s", t.Name())
		return
	}

	tx := newTx(ctx, name, oNodes, udpAddr, intf, timeout)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, udpAddr, intf, timeout)
	t.SetRx(rx)
}
