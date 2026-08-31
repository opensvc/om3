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

	// Loader encrypts and decrypts with the crypto current at call time,
	// instead of the one current when it was created. Users outliving a
	// heartbeat secret rotation, like a hb.ucast connection, must go
	// through it: a rotation is only seamless for those decrypting with
	// the up to date secret, which knows both the previous and the next
	// key.
	Loader struct {
		p *atomic.Pointer[omcrypto.T]
	}

	contextKey int
)

var (
	// assert Loader implements the omcrypto.EncryptDecrypter interface
	_ = omcrypto.EncryptDecrypter(Loader{})
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

// LoaderFromContext returns a Loader on the context crypto
func LoaderFromContext(ctx context.Context) Loader {
	return Loader{p: CryptoFromContext(ctx)}
}

func (t Loader) DecryptWithNode(data []byte) ([]byte, string, error) {
	return t.p.Load().DecryptWithNode(data)
}

func (t Loader) Decrypt(data []byte) ([]byte, error) {
	return t.p.Load().Decrypt(data)
}

func (t Loader) Encrypt(data []byte) ([]byte, error) {
	return t.p.Load().Encrypt(data)
}
