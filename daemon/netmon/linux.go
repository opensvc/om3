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

func (t *Manager) worker() {
	t.log.Infof("starting netlink monitor")

	// Use high-level netlink subscription API
	addrUpdates := make(chan netlink.AddrUpdate, 100)
	linkUpdates := make(chan netlink.LinkUpdate, 100)

	t.lastPublished = make(map[string]linkPublishState)

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
