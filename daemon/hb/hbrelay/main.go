/*
Package hbrelay uses a tiers opensvc agent as a kv store to exchange node data.
*/
package hbrelay

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		hbcfg.T
	}

	cfg struct {
		relay        string
		username     string
		password     string
		passwordFrom datarecv.KeyMeta
		insecure     bool

		timeout  time.Duration
		interval time.Duration

		id  string
		log *plog.Logger
		cli *client.T
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
	log := plog.NewDefaultLogger().Attr("pkg", "daemon/hb/hbrelay").Attr("hb_name", t.Name()).WithPrefix("daemon: hb: relay: " + t.Name() + ": configure: ")
	timeout := t.GetDuration("timeout", 9*time.Second)
	interval := t.GetDuration("interval", 4*time.Second)
	if timeout < 2*interval+1*time.Second {
		oldTimeout := timeout
		timeout = interval*2 + 1*time.Second
		log.Warnf("reajust timeout: %s => %s (<interval>*2+1s)", oldTimeout, timeout)
	}
	relay := t.GetString("relay")
	if relay == "" {
		log.Errorf("no %s.relay is not set in node.conf", t.Name())
		return
	}
	username := t.GetString("username")
	passwordLine := t.GetString("password")
	passKM, err := datarecv.ParseKeyMetaRelWithFallback(passwordLine, naming.NsSys, "password")
	if err != nil {
		log.Errorf("no %s.password parsing: %s", t.Name(), err)
		return
	}

	insecure := t.GetBool("insecure")
	nodes := t.GetStrings("nodes")
	if len(nodes) == 0 {
		k := key.T{Section: "cluster", Option: "nodes"}
		nodes = t.Config().GetStrings(k)
	}
	oNodes := hostname.OtherNodes(nodes)
	log.Tracef("timeout=%s interval=%s relay=%s insecure=%t nodes=%s onodes=%s", timeout, interval, relay, insecure, nodes, oNodes)
	t.SetNodes(oNodes)
	t.SetTimeout(timeout)
	cfg := cfg{
		relay:        relay,
		username:     username,
		passwordFrom: passKM,
		insecure:     insecure,
		timeout:      timeout,
		interval:     interval,
	}
	signature := fmt.Sprintf("type: hb.relay nodes: %s cfg: %s", nodes, cfg.signature())
	t.SetSignature(signature)
	log.Tracef("signature: [%s]", signature)
	name := t.Name()
	tx := newTx(ctx, name, oNodes, cfg)
	t.SetTx(tx)
	rx := newRx(ctx, name, oNodes, cfg)
	t.SetRx(rx)
}

// drain reads and discards all data from the provided ReadCloser and closes
// it to release resources and help Go to recycle the socket.
func drain(rc io.ReadCloser, l *plog.Logger) {
	_, _ = io.Copy(io.Discard, rc)
	if err := rc.Close(); err != nil {
		l.Warnf("drain: %s", err)
	}
}

func (t *cfg) startSubscription(ctx context.Context) *pubsub.Subscription {
	sub := pubsub.SubFromContext(ctx, "daemon.relay."+t.id, pubsub.WithQueueSize(1024))
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, pubsub.Label{"path", t.passwordFrom.Path.String()}, pubsub.Label{"node", hostname.Hostname()})
	sub.Start()
	return sub
}

func (t *cfg) onEvent(ev any) {
	switch c := ev.(type) {
	case *msgbus.AuditStart:
		t.log.HandleAuditStart(c.Q, c.Subsystems, "hb", strings.Replace(t.id, "hb#", "hb:", 1))
	case *msgbus.AuditStop:
		t.log.HandleAuditStop(c.Q, c.Subsystems, "hb", strings.Replace(t.id, "hb#", "hb:", 1))
	case *msgbus.InstanceConfigUpdated:
		if err := t.refreshClient(); err != nil {
			t.log.Errorf("refresh client on changed %s: %s", t.passwordFrom.Path, err)
			return
		}
	}
}

func (t *cfg) attachActiveAuditIfAny(ctx context.Context) {
	reg := daemonctx.AuditRegistry(ctx)
	if reg == nil {
		return
	}
	sess, ok := reg.Snapshot()
	if !ok {
		return
	}
	t.log.HandleAuditStart(sess.Q, sess.Subsystems, "hb", strings.Replace(t.id, "hb#", "hb:", 1))
}

func (t *cfg) refreshClient() error {
	if b, err := t.passwordFrom.Decode(); err != nil {
		return fmt.Errorf("decode password: %w", err)
	} else if string(b) != t.password {
		if t.password != "" {
			t.log.Debugf("password changed for %s", t.passwordFrom.Path)
		}
		t.password = string(b)
		cli, err := client.New(
			client.WithURL(t.relay),
			client.WithUsername(t.username),
			client.WithPassword(t.password),
			client.WithInsecureSkipVerify(t.insecure),
		)
		if err != nil {
			return fmt.Errorf("new client: %w", err)
		} else if cli == nil {
			return fmt.Errorf("unexpected nil client")
		}
		t.cli = cli
		return nil
	}
	t.log.Debugf("password unhanged for %s", t.passwordFrom.Path)
	return nil
}

func (t *cfg) signature() string {
	return fmt.Sprintf("relay: %s username: %s passwordFrom: %s timeout: %s interval: %s insecure: %v",
		t.relay, t.username, t.passwordFrom, t.timeout, t.interval, t.insecure)
}
