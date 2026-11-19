//go:build linux

package netmon

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// nlMsg returns a netlink message of type typ holding data.
func nlMsg(typ uint16, data []byte) []byte {
	b := make([]byte, unix.NLMSG_HDRLEN, unix.NLMSG_HDRLEN+len(data))
	binary.NativeEndian.PutUint32(b[0:4], uint32(unix.NLMSG_HDRLEN+len(data)))
	binary.NativeEndian.PutUint16(b[4:6], typ)
	return append(b, data...)
}

// addrMsg returns an address message of an address on the link of index,
// labeled label, ipv4 with a label, ipv6 without as the kernel sends them.
func addrMsg(typ uint16, ip string, prefixlen, scope, index int, label string) []byte {
	a := net.ParseIP(ip)
	family := unix.AF_INET6
	if a.To4() != nil {
		a = a.To4()
		family = unix.AF_INET
	}
	m := nl.NewIfAddrmsg(family)
	m.Prefixlen = uint8(prefixlen)
	m.Scope = uint8(scope)
	m.Index = uint32(index)
	data := m.Serialize()
	data = append(data, nl.NewRtAttr(unix.IFA_ADDRESS, a).Serialize()...)
	if family == unix.AF_INET {
		data = append(data, nl.NewRtAttr(unix.IFA_LOCAL, a).Serialize()...)
		data = append(data, nl.NewRtAttr(unix.IFA_LABEL, nl.ZeroTerminated(label)).Serialize()...)
	}
	return nlMsg(typ, data)
}

// nsidMsg returns a namespace id message about nsid.
func nsidMsg(typ uint16, nsid int) []byte {
	data := make([]byte, unix.NLMSG_ALIGNTO)
	data[0] = unix.AF_UNSPEC
	v := make([]byte, 4)
	binary.NativeEndian.PutUint32(v, uint32(int32(nsid)))
	return nlMsg(typ, append(data, nl.NewRtAttr(unix.NETNSA_NSID, v).Serialize()...))
}

// nsidOob returns the control message the kernel sends with a message of
// the namespace of id nsid to an all-nsid socket.
func nsidOob(nsid int) []byte {
	b := make([]byte, unix.CmsgSpace(4))
	h := (*unix.Cmsghdr)(unsafe.Pointer(&b[0]))
	h.Level = unix.SOL_NETLINK
	h.Type = unix.NETLINK_LISTEN_ALL_NSID
	h.SetLen(unix.CmsgLen(4))
	binary.NativeEndian.PutUint32(b[unix.CmsgLen(0):], uint32(int32(nsid)))
	return b
}

// The address events of another namespace are kept, the ipv4 ones named
// after their label, the ipv6 ones, which have none, after the namespace id
// and the link index.
func TestParseNsidDatagramOtherNamespace(t *testing.T) {
	d, err := parseNsidDatagram(addrMsg(unix.RTM_NEWADDR, "10.0.0.5", 24, unix.RT_SCOPE_UNIVERSE, 3, "eth0"), nsidOob(6), -1)
	require.NoError(t, err)
	assert.Equal(t, []nsAddrEvent{{nsid: 6, added: true, linkIndex: 3, linkName: "eth0", address: "10.0.0.5/24"}}, d.events)

	d, err = parseNsidDatagram(addrMsg(unix.RTM_DELADDR, "fd00::5", 64, unix.RT_SCOPE_UNIVERSE, 3, ""), nsidOob(6), -1)
	require.NoError(t, err)
	assert.Equal(t, []nsAddrEvent{{nsid: 6, added: false, linkIndex: 3, linkName: "nsid6-index-3", address: "fd00::5/64"}}, d.events)

	d, err = parseNsidDatagram(addrMsg(unix.RTM_NEWADDR, "10.0.0.5", 24, unix.RT_SCOPE_UNIVERSE, 3, "eth0"), nsidOob(0), -1)
	require.NoError(t, err)
	assert.Len(t, d.events, 1, "nsid 0 is another namespace")
}

// The address events of the namespace of the daemon are dropped, the
// subscription of worker publishing them: without control message, with an
// id below zero as older kernels send, or with the id the namespace of the
// daemon holds for itself.
func TestParseNsidDatagramOwnNamespace(t *testing.T) {
	msg := addrMsg(unix.RTM_NEWADDR, "10.0.0.5", 24, unix.RT_SCOPE_UNIVERSE, 3, "eth0")
	for name, oob := range map[string][]byte{"no control message": nil, "nsid -1": nsidOob(-1), "self": nsidOob(2)} {
		d, err := parseNsidDatagram(msg, oob, 2)
		require.NoError(t, err, name)
		assert.Empty(t, d.events, name)
	}
}

// The loopback and link local addresses of a container are no address of a
// resource.
func TestParseNsidDatagramGlobalScopeOnly(t *testing.T) {
	for _, scope := range []int{unix.RT_SCOPE_HOST, unix.RT_SCOPE_LINK} {
		d, err := parseNsidDatagram(addrMsg(unix.RTM_NEWADDR, "fe80::1", 64, scope, 3, ""), nsidOob(6), -1)
		require.NoError(t, err)
		assert.Empty(t, d.events)
	}
}

// The deletion of a namespace id is the one of the namespace of the daemon
// only: another namespace deleting an id it holds says nothing of the ids
// the namespace of the daemon holds.
func TestParseNsidDatagramDeletedNsid(t *testing.T) {
	d, err := parseNsidDatagram(nsidMsg(unix.RTM_DELNSID, 6), nil, -1)
	require.NoError(t, err)
	assert.Equal(t, []int{6}, d.deleted)

	d, err = parseNsidDatagram(nsidMsg(unix.RTM_DELNSID, 0), nsidOob(6), -1)
	require.NoError(t, err)
	assert.Empty(t, d.deleted)
}

type pubSpy struct {
	msgs []pubsub.Messager
}

func (p *pubSpy) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	p.msgs = append(p.msgs, m)
}

// An event repeating the last one of the same address is not published
// again, as an ipv6 address added is said twice. The last events of a
// namespace are forgotten as its id is deleted: the next namespace getting
// the id again, as a container restarting, adds the same address with no
// delete in between.
func TestHandleNsDatagram(t *testing.T) {
	spy := &pubSpy{}
	m := newTestManager(context.Background())
	m.publisher = spy
	m.localhost = "node1"
	state := make(map[int]map[string]bool)
	added := nsAddrEvent{nsid: 6, added: true, linkIndex: 3, linkName: "eth0", address: "10.0.0.5/24"}

	m.handleNsDatagram(state, nsDatagram{events: []nsAddrEvent{added, added}})
	require.Len(t, spy.msgs, 1, "the repeated event is not published")
	msg, ok := spy.msgs[0].(*msgbus.NetIPAddrAdded)
	require.True(t, ok)
	require.NotNil(t, msg.NSID)
	assert.Equal(t, 6, *msg.NSID)
	assert.Equal(t, "10.0.0.5/24", msg.Address)

	other := added
	other.nsid = 7
	m.handleNsDatagram(state, nsDatagram{events: []nsAddrEvent{other}})
	assert.Len(t, spy.msgs, 2, "the same address in another namespace is another address")

	m.handleNsDatagram(state, nsDatagram{deleted: []int{6}, events: []nsAddrEvent{added}})
	assert.Len(t, spy.msgs, 3, "the namespace of id 6 went: the same address in the next one is published")

	deleted := added
	deleted.added = false
	m.handleNsDatagram(state, nsDatagram{events: []nsAddrEvent{deleted}})
	require.Len(t, spy.msgs, 4)
	_, ok = spy.msgs[3].(*msgbus.NetIPAddrDeleted)
	assert.True(t, ok)
	assert.Empty(t, state[6], "a deleted address is forgotten")
}

// Events lost, the addresses seen are forgotten: the deletion of a namespace
// id may be among them, and the next namespace getting the id publishes its
// first address.
func TestForgetNsStateOnLoss(t *testing.T) {
	spy := &pubSpy{}
	m := newTestManager(context.Background())
	m.publisher = spy
	m.localhost = "node1"
	state := make(map[int]map[string]bool)
	added := nsAddrEvent{nsid: 6, added: true, linkIndex: 3, linkName: "eth0", address: "10.0.0.5/24"}
	m.handleNsDatagram(state, nsDatagram{events: []nsAddrEvent{added}})
	forgetNsState(state)
	m.handleNsDatagram(state, nsDatagram{events: []nsAddrEvent{added}})
	assert.Len(t, spy.msgs, 2, "published again once the state is forgotten")
}

// A socket that failed is closed and opened again, after a delay and again
// until it opens; a stop ends the attempts.
func TestReopenNsidSocket(t *testing.T) {
	savedOpen, savedDelay := openNsidSocketFunc, resubscribeDelay
	t.Cleanup(func() { openNsidSocketFunc, resubscribeDelay = savedOpen, savedDelay })
	resubscribeDelay = time.Millisecond
	calls := 0
	openNsidSocketFunc = func() (int, error) {
		calls++
		if calls < 3 {
			return -1, errors.New("not yet")
		}
		return 42, nil
	}
	m := newTestManager(context.Background())
	fd, ok := m.reopenNsidSocket(-1)
	assert.True(t, ok)
	assert.Equal(t, 42, fd)
	assert.Equal(t, 3, calls, "opened again until it opens")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m = newTestManager(ctx)
	_, ok = m.reopenNsidSocket(-1)
	assert.False(t, ok, "a stop ends the attempts")
}
