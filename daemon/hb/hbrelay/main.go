/*
Package hbrelay uses a tiers opensvc agent as a kv store to exchange node data.
*/
package hbrelay

import (
	"context"
	"fmt"
	"os"
	"time"

	"opensvc.com/opensvc/core/hbcfg"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

type (
	T struct {
		hbcfg.T
	}

	capsule struct {
		Updated time.Time `json:"updated"`
		Msg     []byte    `json:"msg"`
	}
	peerConfigs map[string]peerConfig
	peerConfig  struct {
		Slot int
	}
	device struct {
		file *os.File
	}
)

func New() hbcfg.Confer {
	t := &T{}
	var i interface{} = t
	return i.(hbcfg.Confer)
}

func init() {
	hbcfg.Register("relay", New)
}

// Configure implements the Configure function of Confer interface for T
func (t *T) Configure(ctx context.Context) {
	log := daemonlogctx.Logger(ctx).With().Str("type", t.Name()).Logger()
	timeout := t.GetDuration("timeout", 9*time.Second)
	interval := t.GetDuration("interval", 4*time.Second)
	if timeout < 2*interval+1*time.Second {
		oldTimeout := timeout
		timeout = interval*2 + 1*time.Second
		log.Warn().Msgf("reajust timeout: %s => %s (<interval>*2+1s)", oldTimeout, timeout)
	}
	relay := t.GetString("relay")
	if relay == "" {
		log.Error().Msgf("no %s.relay is not set in node.conf", t.Name())
		return
	}
	username := t.GetString("username")
	password, err := t.password()
	if err != nil {
		log.Error().Err(err).Msgf("no %s.password parsing", t.Name())
		return
	}
	insecure := t.GetBool("insecure")
	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Debug().Msgf("configure %s, timeout=%s interval=%s relay=%s insecure=%t nodes=%s onodes=%s", t.Name(), timeout, interval, relay, insecure, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	signature := fmt.Sprintf("type: hb.relay nodes: %s relay: %s timeout: %s interval: %s", nodes, relay, timeout, interval)
	t.SetSignature(signature)
	name := t.Name()
	tx := newTx(ctx, name, oNodes, relay, username, password, insecure, timeout, interval)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, relay, username, password, insecure, timeout, interval)
	t.SetRx(rx)
}

func (t *T) passwordSec() (object.Sec, error) {
	secName := t.GetString("password")
	secPath, err := path.Parse(secName)
	if err != nil {
		return nil, err
	}
	return object.NewSec(secPath, object.WithVolatile(true))
}

func (t *T) password() (string, error) {
	sec, err := t.passwordSec()
	if err != nil {
		return "", err
	}
	b, err := sec.DecodeKey("password")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
