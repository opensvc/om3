//go:build linux

package netmon

import (
	"fmt"
	"net"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// resubscribeDelay is the wait before subscribing again to the link and
// address events, after a subscription ended on a receive error.
var resubscribeDelay = time.Second

func (t *Manager) worker() {
	t.log.Infof("starting netlink monitor")

	for {
		// What was published before the subscription ended is
		// forgotten: the events missed in between may have removed and
		// added back the same address or link, whose next event would
		// else be taken for a repeat and not published.
		t.lastPublished = make(map[string]linkPublishState)

		// done ends both subscriptions: the one still running once the
		// other ended, or both as the daemon stops.
		done := make(chan struct{})
		addrUpdates := make(chan netlink.AddrUpdate, 100)
		linkUpdates := make(chan netlink.LinkUpdate, 100)
		onError := func(err error) {
			t.log.Warnf("netlink subscription: %s", err)
		}
		if err := netlink.AddrSubscribeWithOptions(addrUpdates, done, netlink.AddrSubscribeOptions{ErrorCallback: onError}); err != nil {
			close(done)
			t.log.Errorf("failed to subscribe to address updates: %s", err)
			return
		}
		if err := netlink.LinkSubscribeWithOptions(linkUpdates, done, netlink.LinkSubscribeOptions{ErrorCallback: onError}); err != nil {
			close(done)
			t.log.Errorf("failed to subscribe to link updates: %s", err)
			return
		}
		t.log.Infof("netlink monitor subscribed to link and address events")

		ended := t.receive(addrUpdates, linkUpdates)
		close(done)
		if !ended {
			t.log.Infof("context done, stopping netlink monitor")
			return
		}
		// A subscription ends on a receive error, its channel closed, as
		// on a receive buffer overflow: the events are subscribed to
		// again rather than read from a closed channel for good.
		t.log.Warnf("netlink subscription ended: subscribe again in %s, the events in between are missed", resubscribeDelay)
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(resubscribeDelay):
		}
	}
}

// receive handles the link and address events until the context is done,
// returning false, or until a subscription ends, its channel closed,
// returning true.
func (t *Manager) receive(addrUpdates <-chan netlink.AddrUpdate, linkUpdates <-chan netlink.LinkUpdate) bool {
	for {
		select {
		case <-t.ctx.Done():
			return false
		case update, ok := <-addrUpdates:
			if !ok {
				return true
			}
			t.handleAddrUpdate(update)
		case update, ok := <-linkUpdates:
			if !ok {
				return true
			}
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

	lastPub, exists := t.lastPublished[addrKey]

	// Determine if this is an add or delete
	isAdded := update.NewAddr

	if exists && lastPub.isUp == isAdded {
		// Same operation as last published, skip
		t.log.Debugf("address %s on %s: duplicate operation event (isAdded=%t)",
			update.LinkAddress.String(), linkName, isAdded)
		return
	}

	// Publish (operation changed or it's the first event)

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
	t.lastPublished[addrKey] = linkPublishState{
		isUp:        isAdded,
		operState:   0, // Not used for addresses
		publishedAt: time.Now(),
	}

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
	lastPub, exists := t.lastPublished[linkName]

	if exists && lastPub.isUp == !isDown && lastPub.operState == operState {
		// Same state as last published and not debounced, skip
		t.log.Debugf("link %s: duplicate state event (isUp=%t, oper_state=%d)",
			linkName, !isDown, operState)
		return
	}

	// Publish (state changed or first event)

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
	t.lastPublished[linkName] = linkPublishState{
		isUp:        !isDown,
		operState:   operState,
		publishedAt: time.Now(),
	}

	t.log.Infof("link %s: %s (index %d)", eventType, linkName, linkIndex)
	t.publisher.Pub(msg, t.labelLocalhost)
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
