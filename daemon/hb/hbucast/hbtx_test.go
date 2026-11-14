package hbucast

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSendWorkerCloseConn verifies the mechanism Stop relies on to not wait
// a timeout for a worker parked in a send to a peer that stopped reading.
func TestSendWorkerCloseConn(t *testing.T) {
	t.Run("closing interrupts a blocked write", func(t *testing.T) {
		peer, local := net.Pipe()
		defer peer.Close()

		w := &sendWorker{queue: make(chan sendRequest, 1)}
		w.setConn(local)

		// net.Pipe is synchronous: this write blocks until the peer reads,
		// as a write to a peer whose receive window is full does.
		errC := make(chan error, 1)
		go func() {
			_, err := w.getConn().Write([]byte("a message\x00"))
			errC <- err
		}()

		select {
		case err := <-errC:
			t.Fatalf("the write returned before the peer read: %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		w.closeConn()

		select {
		case err := <-errC:
			require.Error(t, err, "the interrupted write must fail")
		case <-time.After(time.Second):
			t.Fatal("closeConn did not interrupt the blocked write")
		}
		require.Nil(t, w.getConn(), "a closed connection must not be handed out again")
	})

	t.Run("closing twice is harmless", func(t *testing.T) {
		peer, local := net.Pipe()
		defer peer.Close()

		w := &sendWorker{queue: make(chan sendRequest, 1)}
		// the worker closing on its way out and Stop closing to interrupt it
		// can both happen
		w.closeConn()
		w.setConn(local)
		w.closeConn()
		w.closeConn()
		require.Nil(t, w.getConn())
	})
}

// stubAddrs resolves every name to ips, and gives the node the addresses
// held.
func stubAddrs(t *testing.T, ips []string, held []string) {
	t.Helper()
	savedLookup, savedIfaces := lookupIPAddr, interfaceAddrs
	t.Cleanup(func() { lookupIPAddr, interfaceAddrs = savedLookup, savedIfaces })
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		l := make([]net.IPAddr, 0, len(ips))
		for _, s := range ips {
			l = append(l, net.IPAddr{IP: net.ParseIP(s)})
		}
		return l, nil
	}
	interfaceAddrs = func() ([]net.Addr, error) {
		l := make([]net.Addr, 0, len(held))
		for _, s := range held {
			l = append(l, &net.IPNet{IP: net.ParseIP(s), Mask: net.CIDRMask(64, 128)})
		}
		return l, nil
	}
}

// An addr given by its name is sent from: the addresses it resolves to that
// the node holds. It was sent from no address, so from a temporary IPv6
// address the kernel picked, which the peers refused.
func TestDefaultLocalIPsOfAName(t *testing.T) {
	stubAddrs(t, []string{"2001:db8::10", "192.0.2.10", "198.51.100.7"}, []string{"2001:db8::10", "2001:db8::abcd:1234", "192.0.2.10"})
	tx := &tx{ctx: context.Background(), addr: "node1-hb"}
	ips, err := tx.defaultLocalIPs()
	require.NoError(t, err)
	require.Equal(t, "2001:db8::10,192.0.2.10", ips.String())
}

// An addr given by its address is sent from, with no lookup.
func TestDefaultLocalIPsOfAnAddress(t *testing.T) {
	stubAddrs(t, nil, nil)
	tx := &tx{ctx: context.Background(), addr: "192.0.2.10"}
	ips, err := tx.defaultLocalIPs()
	require.NoError(t, err)
	require.Equal(t, "192.0.2.10", ips.String())
}

// A name resolving to no address the node holds, as behind a nat, is sent
// from no address in particular, as before, warned about once.
func TestDefaultLocalIPsOfANameNotHeld(t *testing.T) {
	stubAddrs(t, []string{"198.51.100.7"}, []string{"192.0.2.10"})
	tx := &tx{ctx: context.Background(), addr: "node1-hb"}
	ips, err := tx.defaultLocalIPs()
	require.NoError(t, err)
	require.Empty(t, ips)
	require.Equal(t, "198.51.100.7", tx.addrNotLocal.String(), "warned about")
}

// A peer is dialed from the source of its family: an IPv4 peer from the
// IPv4 source, though an IPv6 one comes first. A source of the family of no
// peer address, an IPv6 source for an IPv4 peer, would fail the dial: the
// peer is dialed from the address the kernel picks then.
func TestDialPeerFromTheSourceOfItsFamily(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	addr := ln.Addr().String()
	ctx := context.Background()

	conn, err := dialPeer(ctx, time.Second, ipList{net.ParseIP("::1"), net.ParseIP("127.0.0.1")}, addr)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", conn.LocalAddr().(*net.TCPAddr).IP.String())
	conn.Close()

	conn, err = dialPeer(ctx, time.Second, ipList{net.ParseIP("::1")}, addr)
	require.NoError(t, err, "no source of the family of the peer: unbound")
	conn.Close()

	conn, err = dialPeer(ctx, time.Second, nil, addr)
	require.NoError(t, err)
	conn.Close()
}

// A peer address that does not answer does not hold its next address for
// the whole timeout: the next one is tried after peerAttemptDelay. The first
// address, of TEST-NET-1, is routed nowhere.
func TestDialPeerTriesTheNextAddressOfAPeer(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	stubAddrs(t, []string{"192.0.2.1", "127.0.0.1"}, nil)
	saved := peerAttemptDelay
	peerAttemptDelay = 50 * time.Millisecond
	t.Cleanup(func() { peerAttemptDelay = saved })

	begin := time.Now()
	conn, err := dialPeer(context.Background(), 5*time.Second, ipList{net.ParseIP("127.0.0.1")}, net.JoinHostPort("node2", port))
	require.NoError(t, err)
	conn.Close()
	require.Less(t, time.Since(begin), 2*time.Second)
}
