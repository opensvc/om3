//go:build linux

package netmon

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	// nsAddrEvent is an address added to or deleted from a link of another
	// network namespace, as the one of a container.
	nsAddrEvent struct {
		nsid      int
		added     bool
		linkIndex int
		linkName  string
		address   string
	}

	// nsDatagram is what a datagram of the all-nsid socket says: the
	// address events of the other namespaces, and the ids of the
	// namespaces this one no longer holds, as their namespace went.
	nsDatagram struct {
		events  []nsAddrEvent
		deleted []int
	}
)

const (
	// nsidReceiveBufferSize is the receive buffer of the all-nsid socket,
	// which hears every container of the node: the default one overflows
	// on a burst, as at the start of many containers.
	nsidReceiveBufferSize = 1 << 20

	// nsidReceiveTimeout bounds a receive, for the listener to see the
	// daemon stopping.
	nsidReceiveTimeout = time.Second
)

// nsidWorker publishes the address events of the other network namespaces
// of the node, as the ones of the containers an ip.netns or ip.cni resource
// configures its address in: the subscription of worker hears the
// namespace of the daemon only.
//
// A socket listening to every namespace the namespace of the daemon holds an
// id for hears them: moving a link into a namespace, as the ip drivers do,
// gives it one. The socket hears the events of the namespace of the daemon
// too, which worker publishes: they are dropped here. The listener is a
// second socket rather than a replacement of the subscription of worker, for
// a burst of container events overflowing its buffer not to lose an event
// of the node.
//
// Without the option, as on a kernel older than 4.2 or a daemon without
// CAP_NET_BROADCAST, the other namespaces are not heard, as before.
func (t *Manager) nsidWorker() {
	fd, err := openNsidSocketFunc()
	if err != nil {
		t.log.Infof("the address events of the other network namespaces are not monitored: %s", err)
		return
	}
	defer func() {
		if fd >= 0 {
			unix.Close(fd)
		}
	}()

	// The id the namespace of the daemon has for itself, if any: its
	// own events would carry it.
	self := -1
	if id, err := netlink.GetNetNsIdByPid(os.Getpid()); err == nil {
		self = id
	}
	t.log.Infof("netlink monitor subscribed to the address events of the other network namespaces")

	buf := make([]byte, 1<<16)
	oob := make([]byte, unix.CmsgSpace(4))
	state := make(map[int]map[string]bool)
	for {
		if t.ctx.Err() != nil {
			return
		}
		n, oobn, _, from, err := unix.Recvmsg(fd, buf, oob, 0)
		switch {
		case err == nil:
		case errors.Is(err, unix.EAGAIN), errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.ENOBUFS):
			t.log.Warnf("address events of the other network namespaces lost: receive buffer overflow")
			forgetNsState(state)
			continue
		default:
			// The socket may be unusable: it is opened again, as the
			// host subscription is subscribed again.
			t.log.Warnf("address events of the other network namespaces: receive: %s: open the socket again in %s, the events in between missed", err, resubscribeDelay)
			forgetNsState(state)
			var ok bool
			if fd, ok = t.reopenNsidSocket(fd); !ok {
				return
			}
			continue
		}
		if sa, ok := from.(*unix.SockaddrNetlink); !ok || sa.Pid != 0 {
			// Not from the kernel.
			continue
		}
		d, err := parseNsidDatagram(buf[:n], oob[:oobn], self)
		if err != nil {
			t.log.Debugf("address events of the other network namespaces: %s", err)
			continue
		}
		t.handleNsDatagram(state, d)
	}
}

// openNsidSocketFunc opens the all-namespace socket: a variable for the
// tests to stub.
var openNsidSocketFunc = openNsidSocket

// reopenNsidSocket closes the socket fd and opens a new one, after
// resubscribeDelay and again until it opens, or the manager stops: false,
// and no socket left open.
func (t *Manager) reopenNsidSocket(fd int) (int, bool) {
	if fd >= 0 {
		unix.Close(fd)
	}
	for {
		select {
		case <-t.ctx.Done():
			return -1, false
		case <-time.After(resubscribeDelay):
		}
		nfd, err := openNsidSocketFunc()
		if err == nil {
			t.log.Infof("netlink monitor subscribed again to the address events of the other network namespaces")
			return nfd, true
		}
		t.log.Warnf("address events of the other network namespaces: open the socket again: %s", err)
	}
}

// forgetNsState forgets the addresses seen in every namespace, as events
// were lost: a namespace id deletion among them would leave the state of the
// namespace gone to the next one getting its id, whose first address would
// be taken for a repeat and not published.
func forgetNsState(state map[int]map[string]bool) {
	clear(state)
}

// openNsidSocket returns a route netlink socket hearing the address events
// and the namespace id changes of every namespace the namespace of the
// daemon holds an id for.
func openNsidSocket() (int, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_ROUTE)
	if err != nil {
		return -1, fmt.Errorf("socket: %w", err)
	}
	fail := func(what string, err error) (int, error) {
		unix.Close(fd)
		return -1, fmt.Errorf("%s: %w", what, err)
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fail("bind", err)
	}
	for _, group := range []int{unix.RTNLGRP_IPV4_IFADDR, unix.RTNLGRP_IPV6_IFADDR, unix.RTNLGRP_NSID} {
		if err := unix.SetsockoptInt(fd, unix.SOL_NETLINK, unix.NETLINK_ADD_MEMBERSHIP, group); err != nil {
			return fail(fmt.Sprintf("join group %d", group), err)
		}
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_NETLINK, unix.NETLINK_LISTEN_ALL_NSID, 1); err != nil {
		return fail("listen to all the namespaces", err)
	}
	// The forced size passes the rmem_max limit, as root; else the
	// largest the limit lets.
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, nsidReceiveBufferSize); err != nil {
		_ = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, nsidReceiveBufferSize)
	}
	tv := unix.NsecToTimeval(nsidReceiveTimeout.Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		return fail("receive timeout", err)
	}
	return fd, nil
}

// parseNsidDatagram returns the address events of the other namespaces and
// the namespace ids deleted a datagram of the all-nsid socket holds.
//
// The kernel says the namespace a message comes from with the id the
// namespace of the daemon holds for it, as a control message. A message of
// the namespace of the daemon has none, or one below zero as older kernels
// send, or self, the id the namespace of the daemon holds for itself, if
// any.
//
// An address event of the namespace of the daemon is dropped: worker
// publishes it. The namespace id changes are the ones of the namespace of
// the daemon only: another namespace changing the ids it holds says nothing
// of the ids this one holds.
func parseNsidDatagram(buf, oob []byte, self int) (nsDatagram, error) {
	var d nsDatagram
	nsid, other, err := datagramNsid(oob, self)
	if err != nil {
		return d, err
	}
	msgs, err := syscall.ParseNetlinkMessage(buf)
	if err != nil {
		return d, fmt.Errorf("parse netlink messages: %w", err)
	}
	for _, m := range msgs {
		switch m.Header.Type {
		case unix.RTM_NEWADDR, unix.RTM_DELADDR:
			if !other {
				continue
			}
			ev, ok, err := parseNsAddr(m.Data)
			if err != nil {
				return d, err
			}
			if !ok {
				continue
			}
			ev.nsid = nsid
			ev.added = m.Header.Type == unix.RTM_NEWADDR
			if ev.linkName == "" {
				// An ipv6 address message names no link.
				ev.linkName = fmt.Sprintf("nsid%d-index-%d", nsid, ev.linkIndex)
			}
			d.events = append(d.events, ev)
		case unix.RTM_DELNSID:
			if other {
				continue
			}
			if id, ok := parseNsidMsg(m.Data); ok {
				d.deleted = append(d.deleted, id)
			}
		}
	}
	return d, nil
}

// datagramNsid returns the id of the namespace a datagram comes from, and
// whether it is another namespace than the one of the daemon.
func datagramNsid(oob []byte, self int) (int, bool, error) {
	if len(oob) == 0 {
		return -1, false, nil
	}
	cmsgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return -1, false, fmt.Errorf("parse control messages: %w", err)
	}
	for _, c := range cmsgs {
		if c.Header.Level != unix.SOL_NETLINK || c.Header.Type != unix.NETLINK_LISTEN_ALL_NSID || len(c.Data) < 4 {
			continue
		}
		nsid := int(int32(binary.NativeEndian.Uint32(c.Data[:4])))
		if nsid < 0 || nsid == self {
			return nsid, false, nil
		}
		return nsid, true, nil
	}
	return -1, false, nil
}

// parseNsAddr returns the address event of an address message, false for
// an address not of global scope: the loopback and link local addresses of
// the containers are no address of a resource.
func parseNsAddr(data []byte) (nsAddrEvent, bool, error) {
	var ev nsAddrEvent
	if len(data) < unix.SizeofIfAddrmsg {
		return ev, false, fmt.Errorf("address message of %d bytes", len(data))
	}
	msg := nl.DeserializeIfAddrmsg(data)
	if msg.Scope != unix.RT_SCOPE_UNIVERSE {
		return ev, false, nil
	}
	attrs, err := nl.ParseRouteAttr(data[unix.SizeofIfAddrmsg:])
	if err != nil {
		return ev, false, fmt.Errorf("parse address attributes: %w", err)
	}
	var local, address []byte
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case unix.IFA_ADDRESS:
			address = attr.Value
		case unix.IFA_LOCAL:
			local = attr.Value
		case unix.IFA_LABEL:
			if n := len(attr.Value); n > 0 {
				ev.linkName = string(attr.Value[:n-1])
			}
		}
	}
	// IFA_LOCAL is the address of the link, IFA_ADDRESS its peer on a
	// point to point link: ipv6 sends IFA_ADDRESS only.
	ip := local
	if ip == nil {
		ip = address
	}
	if ip == nil {
		return ev, false, nil
	}
	ev.linkIndex = int(msg.Index)
	ev.address = (&net.IPNet{IP: net.IP(ip), Mask: net.CIDRMask(int(msg.Prefixlen), 8*len(ip))}).String()
	return ev, true, nil
}

// parseNsidMsg returns the namespace id a namespace id message is about.
func parseNsidMsg(data []byte) (int, bool) {
	// The message begins with a rtgenmsg, aligned.
	if len(data) < unix.NLMSG_ALIGNTO {
		return 0, false
	}
	attrs, err := nl.ParseRouteAttr(data[unix.NLMSG_ALIGNTO:])
	if err != nil {
		return 0, false
	}
	for _, attr := range attrs {
		if attr.Attr.Type == unix.NETNSA_NSID && len(attr.Value) >= 4 {
			return int(int32(binary.NativeEndian.Uint32(attr.Value[:4]))), true
		}
	}
	return 0, false
}

// handleNsDatagram publishes the address events of a datagram, but the ones
// repeating the last event of the same address of the same link of the same
// namespace, as an ipv6 address added is said twice.
//
// state holds the last event of each address, by namespace id. The state of
// a namespace id is forgotten as its namespace goes: the addresses it held
// are deleted with no message, and the next namespace getting the id again
// may add the same address on the same link, the one of a container
// restarting.
func (t *Manager) handleNsDatagram(state map[int]map[string]bool, d nsDatagram) {
	for _, nsid := range d.deleted {
		delete(state, nsid)
	}
	for _, ev := range d.events {
		key := fmt.Sprintf("%d:%s", ev.linkIndex, ev.address)
		last, ok := state[ev.nsid]
		if !ok {
			last = make(map[string]bool)
			state[ev.nsid] = last
		}
		if added, ok := last[key]; ok && added == ev.added {
			continue
		}
		// Only the addresses present are kept: a delete forgets its
		// address, for addresses churned in a namespace not to grow
		// the state, a delete repeated being published again, which
		// only refreshes.
		if ev.added {
			last[key] = true
		} else {
			delete(last, key)
		}
		nsid := ev.nsid
		if ev.added {
			t.log.Infof("address added: %s on %s (nsid %d)", ev.address, ev.linkName, nsid)
			t.publisher.Pub(&msgbus.NetIPAddrAdded{
				Node:      t.localhost,
				NSID:      &nsid,
				LinkIndex: ev.linkIndex,
				LinkName:  ev.linkName,
				Address:   ev.address,
			}, t.labelLocalhost)
		} else {
			t.log.Infof("address deleted: %s on %s (nsid %d)", ev.address, ev.linkName, nsid)
			t.publisher.Pub(&msgbus.NetIPAddrDeleted{
				Node:      t.localhost,
				NSID:      &nsid,
				LinkIndex: ev.linkIndex,
				LinkName:  ev.linkName,
				Address:   ev.address,
			}, t.labelLocalhost)
		}
	}
}
