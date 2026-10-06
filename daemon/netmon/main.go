// Package netmon is responsible for monitoring network link and IP address changes
// via netlink and publishing them as pubsub events.
//
// It starts after nmon and waits for node.StatusData.GetByNode(t.localhost) to return
// a non-nil and non-zero value before starting the netlink listener.
//
// The netlink monitor subscribes to RTMGRP_LINK, RTMGRP_IPV4_IFADDR, and RTMGRP_IPV6_IFADDR
// groups to receive real-time notifications of link and address changes, equivalent to
// "ip monitor link address label". Other operating systems have no netlink:
// there, the manager only relays the audit requests.
package netmon

import (
	"context"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	Manager struct {
		drainDuration time.Duration

		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger

		publisher pubsub.Publisher
		databus   *daemondata.T
		sub       *pubsub.Subscription
		subQS     pubsub.QueueSizer

		localhost      string
		labelLocalhost pubsub.Label

		// Track last published state and timestamp for debouncing
		lastPublished map[string]linkPublishState

		wg sync.WaitGroup
	}

	linkPublishState struct {
		isUp        bool
		operState   uint8
		publishedAt time.Time
	}
)

func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/netmon").WithPrefix("daemon: netmon: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
	}
}

// Start launches the netmon worker goroutine
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.databus = daemondata.FromContext(t.ctx)
	t.publisher = pubsub.PubFromContext(t.ctx)

	// Start pubsub subscriptions for audit and other control messages
	t.startSubscriptions()

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer t.log.Infof("worker done")
		t.worker()
	}()

	t.log.Infof("started")
	return nil
}

func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	if t.sub != nil {
		if err := t.sub.Stop(); err != nil {
			t.log.Warnf("subscription stop: %s", err)
		}
	}
	t.wg.Wait()
	return nil
}

// startSubscriptions starts the pubsub subscriptions for control messages like AuditStart/AuditStop
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.netmon", t.subQS)

	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})

	sub.Start()
	t.sub = sub

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			case ev := <-sub.C:
				switch c := ev.(type) {
				case *msgbus.AuditStart:
					t.log.HandleAuditStart(c.Q, c.Subsystems, "netmon")
				case *msgbus.AuditStop:
					t.log.HandleAuditStop(c.Q, c.Subsystems, "netmon")
				}
			}
		}
	}()
}

// shouldIgnoreLinkName checks if a link name should be ignored
func (t *Manager) shouldIgnoreLinkName(linkName string) bool {
	if linkName == "" {
		return true
	}

	// Ignore virtual interfaces (same as "ip monitor" would typically filter)
	virtualPrefixes := []string{"veth", "lo", "docker", "tun", "tap", "ip6tnl", "iptun", "gre", "gretap"}
	for _, prefix := range virtualPrefixes {
		if len(linkName) >= len(prefix) && linkName[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}
