package hbconfig

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/hbsecret"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	T struct {
		name        string
		config      cluster.ConfigHeartbeat
		localhost   string
		clusterName string
		outdatedC   chan bool
		cancel      context.CancelFunc
		started     bool
	}
)

// OutdatedC returns a channel that emits boolean values indicating if the
// previously served values are outdated and can be fetched again.
// Example:
//
//	for {
//		select {
//		case outdated := <-hbconfig.OutdatedC():
//			if outdated {
//				// fetch new secrets and recreate cipher
//				newSecrets := hbconfig.Secrets()
//				refreshCipher(newSecrets)
//			}
//		}
//	}
func (t *T) OutdatedC() <-chan bool {
	return t.outdatedC
}

func (t *T) Nodename() string {
	return t.localhost
}

func (t *T) ClusterName() string {
	if t == nil {
		return ""
	}
	return t.clusterName
}

func (t *T) Secrets() (currentVersion uint64, currentSecret string, nextVersion uint64, nextSecret string) {
	if t == nil {
		return
	}
	return t.config.Secrets()
}

func (t *T) Stop() error {
	if t.cancel == nil {
		return fmt.Errorf("can't stop not started crypto argser")
	}
	t.cancel()
	return nil
}

func New(name string) *T {
	return &T{
		name:      name,
		localhost: hostname.Hostname(),
		outdatedC: make(chan bool),
	}
}

// Start initializes the background process to manage subscription and configuration
// updates for the instance.
// Returns an error if the process fails to start.
func (t *T) Start(ctx context.Context) error {
	if t.started {
		return fmt.Errorf("already started")
	}
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	started := make(chan bool)
	go func() {
		outdated := false
		defer t.cancel()
		sub := pubsub.SubFromContext(ctx, t.name)
		sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
		sub.AddFilter(&msgbus.HeartbeatConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
		sub.Start()
		defer sub.Stop()
		t.clusterName = cluster.ConfigData.Get().Name
		cVersion, cSecret, nVersion, nSecret, err := hbsecret.DecodeSecretAndVersions()
		if err != nil {
			panic(err)
		}
		cfg := cluster.ConfigHeartbeat{}
		cfg.SetCurrent(cVersion, cSecret)
		cfg.SetNext(nVersion, nSecret)
		t.config = *cfg.DeepCopy()

		started <- true
		for {
			select {
			case <-ctx.Done():
				return
			case i := <-sub.C:
				switch m := i.(type) {
				case *msgbus.ClusterConfigUpdated:
					t.clusterName = m.Value.Name
				case *msgbus.HeartbeatConfigUpdated:
					t.config = *m.Value.DeepCopy()
					outdated = true
				}
			case t.outdatedC <- outdated:
				outdated = false
			}
		}
	}()
	<-started
	return nil
}
