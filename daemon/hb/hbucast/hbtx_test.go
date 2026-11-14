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

// An addr given by its name is sent from: its first address, when the node
// holds it. It was sent from no address, so from a temporary IPv6 address
// the kernel picked, which the peers refused.
func TestDefaultLocalIPOfAName(t *testing.T) {
	stubAddrs(t, []string{"2001:db8::10", "192.0.2.10"}, []string{"2001:db8::10", "2001:db8::abcd:1234"})
	tx := &tx{ctx: context.Background(), addr: "node1-hb"}
	ip, err := tx.defaultLocalIP()
	require.NoError(t, err)
	require.Equal(t, "2001:db8::10", ip.String())
}

// An addr given by its address is sent from, with no lookup.
func TestDefaultLocalIPOfAnAddress(t *testing.T) {
	stubAddrs(t, nil, nil)
	tx := &tx{ctx: context.Background(), addr: "192.0.2.10"}
	ip, err := tx.defaultLocalIP()
	require.NoError(t, err)
	require.Equal(t, "192.0.2.10", ip.String())
}

// A name resolving to an address the node does not hold, as behind a nat,
// is sent from no address in particular, as before, warned about once.
func TestDefaultLocalIPOfANameNotHeld(t *testing.T) {
	stubAddrs(t, []string{"198.51.100.7"}, []string{"192.0.2.10"})
	tx := &tx{ctx: context.Background(), addr: "node1-hb"}
	ip, err := tx.defaultLocalIP()
	require.NoError(t, err)
	require.Nil(t, ip)
	require.Equal(t, "198.51.100.7", tx.addrNotLocal.String(), "warned about")
}
