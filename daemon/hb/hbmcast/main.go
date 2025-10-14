// Package hbmcast implement a hb multicast driver
package hbmcast

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/opensvc/om3/core/hbcfg"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/plog"
)

// T is the multicast heartbeat
//
// The maximum size of messages that this heartbeat is able to send and receive depends on
// var Max... values
//
// The maximum size of hb message to send is MaxFragments * MaxChunkSize: 10M
//
// The maximum size of hb message to receive is MaxFragments * MaxDatagramSize: 12M
//
// The maximum total data size from a source is MaxMessages * MaxFragments * MaxDatagramSize: 100M
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
	// TODO define common rule for message length, hb ucast defines msgMaxSize 10*1000*1000)

	// MaxMessages is the maximum number of messages from a source
	MaxMessages = 10

	// MaxFragments is the maximum number of fragments when Tx split the message
	// into fragments.
	MaxFragments = 200

	// MaxChunkSize is the maximum size of chunk in a fragment
	MaxChunkSize = 50 * 1024

	// MaxDatagramSize is the maximum size of a datagram to read
	MaxDatagramSize = 60 * 1024
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
	log := plog.NewDefaultLogger().Attr("ctx", "daemon/hb/hbmcast").Attr("hb_name", t.Name()).WithPrefix("daemon: hb: mcast: " + t.Name() + ": configure: ")
	interval := t.GetDuration("interval", 5*time.Second)
	timeout := t.GetDuration("timeout", 15*time.Second)
	intf := t.GetString("intf")
	port := t.GetInt("port")
	addr := t.GetString("addr")
	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Debugf("timeout=%s interval= %s port=%d nodes=%s onodes=%s", timeout, interval,
		port, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetInterval(interval)
	t.SetTimeout(timeout)
	signature := fmt.Sprintf("type: hb.mcast, port: %d nodes: %s timeout: %s intf: %s interval: %s",
		port, nodes, timeout, intf, interval)
	t.SetSignature(signature)
	name := t.Name()

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		log.Errorf("resolve udp addr: %s", err)
		return
	}

	var ifi *net.Interface
	var laddr *net.UDPAddr

	if intf != "" {
		ifi, err = net.InterfaceByName(intf)
		if err != nil {
			log.Errorf("can't get interface by name: %s", err)
			return
		}
		log.Debugf("set rx interface %s", ifi.Name)

		addrs, err := ifi.Addrs()
		if err != nil {
			log.Warnf("intf %s addrs: %s", ifi.Name, err)
			return
		}
		for _, addr := range addrs {
			addrStr := addr.String()
			l := strings.Split(addrStr, "/")
			laddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", l[0], 0))
			if err != nil {
				log.Debugf("intf %s make tx laddr from addr %s: %s", ifi.Name, addr, err)
			} else {
				break
			}
		}
		log.Debugf("set tx interface %s laddr %s", ifi.Name, laddr)
	}

	tx := newTx(ctx, name, oNodes, laddr, udpAddr, timeout, interval)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, udpAddr, ifi, timeout)
	t.SetRx(rx)
}
