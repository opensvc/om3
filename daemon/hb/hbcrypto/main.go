// Package hbcrypto handles creation and updates of *atomic.Pointer[omcrypto.T]
// to follow the cluster name or hb secret object changes.
package hbcrypto

import (
	"context"
	"sync/atomic"

	"github.com/opensvc/om3/v3/core/hbsecret"
	"github.com/opensvc/om3/v3/core/omcrypto"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		clusterName string
		nodename    string

		cancel context.CancelFunc
	}

	contextKey int
)

const (
	cryptoKey contextKey = 0
)

func (t *T) Stop() error {
	if t == nil {
		return nil
	}
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

func (t *T) Start(ctx context.Context, clusterName string, sec hbsecret.Secret) *atomic.Pointer[omcrypto.T] {
	var a atomic.Pointer[omcrypto.T]
	t.clusterName = clusterName
	c := omcrypto.New(hostname.Hostname(), t.clusterName, &sec)
	a.Store(c)

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	started := make(chan bool)
	go func() {
		defer t.cancel()
		sub := pubsub.SubFromContext(ctx, "hbcrypto")
		sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
		sub.AddFilter(&msgbus.HeartbeatSecretUpdated{}, pubsub.Label{"node", hostname.Hostname()})
		sub.Start()
		defer func() { _ = sub.Stop() }()

		started <- true
		for {
			select {
			case <-ctx.Done():
				return
			case i := <-sub.C:
				switch m := i.(type) {
				case *msgbus.ClusterConfigUpdated:
					t.clusterName = m.Value.Name
				case *msgbus.HeartbeatSecretUpdated:
					c := omcrypto.New(hostname.Hostname(), t.clusterName, &m.Value)
					a.Store(c)
				}
			}
		}
	}()
	<-started
	return &a
}

func ContextWithCrypto(ctx context.Context, c *atomic.Pointer[omcrypto.T]) context.Context {
	return context.WithValue(ctx, cryptoKey, c)
}

func CryptoFromContext(ctx context.Context) *atomic.Pointer[omcrypto.T] {
	if c, ok := ctx.Value(cryptoKey).(*atomic.Pointer[omcrypto.T]); ok {
		return c
	}
	panic("context has no crypto")
}
