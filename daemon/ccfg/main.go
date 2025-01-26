// Package ccfg is responsible for the cluster config
//
// It subscribes on msgbus.ConfigFileUpdated for cluster to provide:
//
//	cluster configuration reload:
//	  => cluster.ConfigData update => .cluster.config
//	  => clusternode update (for node selector, clusternodes dereference)
//	  => publication of msgbus.ClusterConfigUpdated for local node
package ccfg

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	Manager struct {
		state       cluster.Config
		networkSigs map[string]string

		ctx           context.Context
		cancel        context.CancelFunc
		drainDuration time.Duration
		pub           pubsub.PublishBuilder
		log           *plog.Logger
		startedAt     time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		scopeNodes  []string
		nodeMonitor map[string]node.Monitor

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		wg sync.WaitGroup
	}

	// NodeDB implements AuthenticateNode
	NodeDB struct{}
)

func New(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	return &Manager{
		drainDuration: drainDuration,
		localhost:     hostname.Hostname(),
		log:           plog.NewDefaultLogger().WithPrefix("daemon: ccfg: ").Attr("pkg", "daemon/ccfg"),
		networkSigs:   make(map[string]string),

		subQS: subQS,
	}
}

// Start launches the ccfg worker goroutine
func (t *Manager) Start(parent context.Context) error {
	t.ctx, t.cancel = context.WithCancel(parent)
	t.pub = pubsub.PubFromContext(t.ctx)

	t.pubClusterConfig()

	t.startSubscriptions()
	t.wg.Add(1)
	go func() {
		defer func() {
			if err := t.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				t.log.Warnf("subscription stop: %s", err)
			}
			t.wg.Done()
		}()
		t.worker()
	}()

	return nil
}

func (t *Manager) Stop() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.ccfg", t.subQS)
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", "cluster"})
	sub.Start()
	t.sub = sub
}

// worker watch for local ccfg updates
func (t *Manager) worker() {
	defer t.log.Debugf("done")

	t.startedAt = time.Now()

	for {
		select {
		case <-t.ctx.Done():
			return
		case i := <-t.sub.C:
			switch c := i.(type) {
			case *msgbus.ConfigFileUpdated:
				t.onConfigFileUpdated(c)
			}
		}
	}
}

// AuthenticateNode returns nil if nodename is a cluster node and password is cluster secret
func (*NodeDB) AuthenticateNode(nodename, password string) error {
	if nodename == "" {
		return fmt.Errorf("can't authenticate: nodename is empty")
	}
	clu := cluster.ConfigData.Get()
	if !clu.Nodes.Contains(nodename) {
		return fmt.Errorf("can't authenticate: %s is not a cluster node", nodename)
	}
	clusterSecret := clu.Secret()
	if clusterSecret == "" {
		return fmt.Errorf("can't authenticate: empty cluster secret")
	}
	if clusterSecret != password {
		return fmt.Errorf("can't authenticate: %s has wrong password", nodename)
	}
	return nil
}
