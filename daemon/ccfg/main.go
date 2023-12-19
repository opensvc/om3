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
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	ccfg struct {
		state       cluster.Config
		networkSigs map[string]string

		clusterConfig *xconfig.T
		ctx           context.Context
		cancel        context.CancelFunc
		drainDuration time.Duration
		bus           *pubsub.Bus
		log           *plog.Logger
		startedAt     time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		scopeNodes  []string
		nodeMonitor map[string]node.Monitor

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription
		wg  sync.WaitGroup
	}

	// NodeDB implements AuthenticateNode
	NodeDB struct{}
)

func New(drainDuration time.Duration) *ccfg {
	o := &ccfg{
		drainDuration: drainDuration,
		localhost:     hostname.Hostname(),
		log:           plog.NewDefaultLogger().WithPrefix("daemon: ccfg: ").Attr("pkg", "daemon/ccfg"),
		networkSigs:   make(map[string]string),
	}
	return o
}

// Start launches the ccfg worker goroutine
func (o *ccfg) Start(parent context.Context) error {
	o.ctx, o.cancel = context.WithCancel(parent)
	o.bus = pubsub.BusFromContext(o.ctx)

	if n, err := object.NewCluster(object.WithVolatile(true)); err != nil {
		return err
	} else {
		o.clusterConfig = n.Config()
	}

	o.pubClusterConfig()

	o.startSubscriptions()
	o.wg.Add(1)
	go func() {
		defer func() {
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Warnf("subscription stop: %s", err)
			}
			o.wg.Done()
		}()
		o.worker()
	}()

	return nil
}

func (o *ccfg) Stop() error {
	o.cancel()
	o.wg.Wait()
	return nil
}

func (o *ccfg) startSubscriptions() {
	sub := o.bus.Sub("ccfg")
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", "cluster"})
	sub.Start()
	o.sub = sub
}

// worker watch for local ccfg updates
func (o *ccfg) worker() {
	defer o.log.Debugf("done")

	o.startedAt = time.Now()

	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case *msgbus.ConfigFileUpdated:
				o.onConfigFileUpdated(c)
			}
		}
	}
}

// AuthenticateNode returns nil if nodename is a cluster node and password is cluster secret
func (_ *NodeDB) AuthenticateNode(nodename, password string) error {
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
