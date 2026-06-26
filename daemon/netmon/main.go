// Package netmon is responsible for monitoring network link and IP address changes
// via netlink and publishing them as pubsub events.
//
// It starts after nmon and waits for node.StatusData.GetByNode(t.localhost) to return
// a non-nil and non-zero value before starting the netlink listener.
//
// The netlink monitor subscribes to RTMGRP_LINK, RTMGRP_IPV4_IFADDR, and RTMGRP_IPV6_IFADDR
// groups to receive real-time notifications of link and address changes, equivalent to
// "ip monitor link address label".
package netmon

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/node"
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
		databus  *daemondata.T
		sub      *pubsub.Subscription
		subQS   pubsub.QueueSizer

		localhost     string
		labelLocalhost pubsub.Label

		// Track last published state and timestamp for debouncing
		lastPublished map[string]linkPublishState
		publishMu     sync.RWMutex

		wg sync.WaitGroup
	}
	
	linkPublishState struct {
		isUp       bool
		operState  uint8
		publishedAt time.Time
	}
)

func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:           plog.NewDefaultLogger().Attr("pkg", "daemon/netmon").WithPrefix("daemon: netmon: "),
		localhost:     localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:         subQS,
		lastPublished: make(map[string]linkPublishState),
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

	// Wait for node.StatusData.GetByNode(t.localhost) to return non-nil and non-zero
	if err := t.waitForNodeStatus(); err != nil {
		return fmt.Errorf("wait for node status: %w", err)
	}

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

// waitForNodeStatus waits for node.StatusData.GetByNode(t.localhost) to return
// a non-nil and non-zero value
func (t *Manager) waitForNodeStatus() error {
	t.log.Infof("waiting for node status data for %s", t.localhost)
	ctx, cancel := context.WithTimeout(t.ctx, t.drainDuration)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for node status data")
		default:
			status := node.StatusData.GetByNode(t.localhost)
			if status != nil && !t.isZeroStatus(*status) {
				t.log.Infof("node status data available for %s", t.localhost)
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
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

// isZeroStatus checks if a node.Status is a zero value
// We only need to check Agent field - if it's empty, the status is considered zero
func (t *Manager) isZeroStatus(status node.Status) bool {
	return status.Agent == ""
}

func (t *Manager) worker() {
	t.log.Infof("starting netlink monitor")

	// Use high-level netlink subscription API
	addrUpdates := make(chan netlink.AddrUpdate, 100)
	linkUpdates := make(chan netlink.LinkUpdate, 100)
	
	// Subscribe to address changes
	if err := netlink.AddrSubscribe(addrUpdates, t.ctx.Done()); err != nil {
		t.log.Errorf("failed to subscribe to address updates: %s", err)
		return
	}
	// No AddrUnsubscribe function - subscription ends when done channel closes or channel is garbage collected

	// Subscribe to link changes
	if err := netlink.LinkSubscribe(linkUpdates, t.ctx.Done()); err != nil {
		t.log.Errorf("failed to subscribe to link updates: %s", err)
		return
	}
	// No LinkUnsubscribe function - subscription ends when done channel closes or channel is garbage collected

	t.log.Infof("netlink monitor subscribed to link and address events")

	for {
		select {
		case <-t.ctx.Done():
			t.log.Infof("context done, stopping netlink monitor")
			return
		case update := <-addrUpdates:
			t.handleAddrUpdate(update)
		case update := <-linkUpdates:
			t.handleLinkUpdate(update)
		}
	}
}

// handleAddrUpdate handles address updates from netlink
func (t *Manager) handleAddrUpdate(update netlink.AddrUpdate) {
	if update.LinkIndex == 0 {
		return
	}
	
	link, err := netlink.LinkByIndex(update.LinkIndex)
	if err != nil {
		t.log.Debugf("failed to get link by index %d: %s", update.LinkIndex, err)
		return
	}
	
	linkName := link.Attrs().Name
	if linkName == "" {
		linkName = fmt.Sprintf("index-%d", update.LinkIndex)
	}
	
	// Check if this is a virtual link we should ignore
	if t.shouldIgnoreLinkName(linkName) {
		t.log.Debugf("ignoring address event for virtual link %s (index %d)", linkName, update.LinkIndex)
		return
	}
	
	// Debounce: track last published address event per (link, address) combination
	addrKey := fmt.Sprintf("%s:%s", linkName, update.LinkAddress.String())
	
	t.publishMu.RLock()
	lastPub, exists := t.lastPublished[addrKey]
	t.publishMu.RUnlock()
	
	// Determine if this is an add or delete
	isAdded := update.NewAddr
	
	// If we published the same operation recently (within 100ms), skip
	if exists && lastPub.isUp == isAdded {
		elapsed := time.Since(lastPub.publishedAt)
		if elapsed < 100*time.Millisecond {
			t.log.Debugf("address %s on %s: debouncing event (same operation=%t, elapsed=%v)", 
				update.LinkAddress.String(), linkName, isAdded, elapsed)
			return
		}
	}
	
	// Only publish if operation changed or it's the first event
	if exists && lastPub.isUp == isAdded {
		// Same operation as last published, skip
		t.log.Debugf("address %s on %s: duplicate operation event (isAdded=%t)", 
			update.LinkAddress.String(), linkName, isAdded)
		return
	}
	
	var eventType string
	var msg pubsub.Messager
	
	if isAdded {
		eventType = "added"
		msg = &msgbus.NetIPAddrAdded{
			Node:      t.localhost,
			LinkIndex: int(update.LinkIndex),
			LinkName:  linkName,
			Address:   update.LinkAddress.String(),
		}
	} else {
		eventType = "deleted"
		msg = &msgbus.NetIPAddrDeleted{
			Node:      t.localhost,
			LinkIndex: int(update.LinkIndex),
			LinkName:  linkName,
			Address:   update.LinkAddress.String(),
		}
	}
	
	// Update last published state for this address on this link
	t.publishMu.Lock()
	t.lastPublished[addrKey] = linkPublishState{
		isUp:       isAdded,
		operState:  0, // Not used for addresses
		publishedAt: time.Now(),
	}
	t.publishMu.Unlock()
	
	t.log.Infof("address %s: %s on %s", eventType, update.LinkAddress.String(), linkName)
	t.publisher.Pub(msg, t.labelLocalhost)
}

// handleLinkUpdate handles link updates from netlink
func (t *Manager) handleLinkUpdate(update netlink.LinkUpdate) {
	linkIndex := update.Index
	if linkIndex == 0 {
		return
	}
	
	linkName := update.Link.Attrs().Name
	if linkName == "" {
		linkName = fmt.Sprintf("index-%d", linkIndex)
	}
	
	// Check if this is a virtual link we should ignore
	if t.shouldIgnoreLinkName(linkName) {
		t.log.Debugf("ignoring link event for virtual link %s (index %d)", linkName, linkIndex)
		return
	}
	
	// Get current flags and operstate
	currentFlags := update.Flags
	operState := uint8(update.Link.Attrs().OperState)
	adminUp := (currentFlags & unix.IFF_UP) != 0
	
	// Determine effective state
	// A link is considered "down" if admin is down OR if it has no carrier (for interfaces that support it)
	isDown := !adminUp || operState == netlink.OperDown || operState == netlink.OperNotPresent || 
		operState == netlink.OperLowerLayerDown || operState == netlink.OperTesting
	
	// Debounce: check if we recently published an event for this link in the same state
	t.publishMu.RLock()
	lastPub, exists := t.lastPublished[linkName]
	t.publishMu.RUnlock()
	
	// If we published the same state recently (within 100ms), skip this event
	if exists && lastPub.isUp == !isDown && lastPub.operState == operState {
		elapsed := time.Since(lastPub.publishedAt)
		if elapsed < 100*time.Millisecond {
			t.log.Debugf("link %s: debouncing event (same state=%t, oper_state=%d, elapsed=%v)", 
				linkName, !isDown, operState, elapsed)
			return
		}
	}
	
	// Only publish if state actually changed (not just a transient event)
	// Always publish the first event, or if state changed from last published
	if exists && lastPub.isUp == !isDown && lastPub.operState == operState {
		// Same state as last published and not debounced, skip
		t.log.Debugf("link %s: duplicate state event (isUp=%t, oper_state=%d)", 
			linkName, !isDown, operState)
		return
	}
	
	// Publish the event
	var eventType string
	var msg pubsub.Messager
	
	if isDown {
		eventType = "down"
		msg = &msgbus.NetLinkDown{
			Node:      t.localhost,
			LinkIndex: int(linkIndex),
			LinkName:  linkName,
		}
	} else {
		eventType = "up"
		msg = &msgbus.NetLinkUp{
			Node:      t.localhost,
			LinkIndex: int(linkIndex),
			LinkName:  linkName,
		}
	}
	
	// Update last published state
	t.publishMu.Lock()
	t.lastPublished[linkName] = linkPublishState{
		isUp:       !isDown,
		operState:  operState,
		publishedAt: time.Now(),
	}
	t.publishMu.Unlock()
	
	t.log.Infof("link %s: %s (index %d)", eventType, linkName, linkIndex)
	t.publisher.Pub(msg, t.labelLocalhost)
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

// GetLocalIPs returns all non-loopback IP addresses assigned to the local node
func GetLocalIPs() ([]net.IP, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	for _, link := range links {
		// Skip loopback
		if link.Attrs().Name == "lo" {
			continue
		}

		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ip := addr.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, ip)
		}
	}

	return ips, nil
}
