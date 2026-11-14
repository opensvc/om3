package httpclientcache

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// localListener returns a listener on 127.0.0.1 and its port.
func localListener(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return port
}

// setVar sets a package variable for the test.
func setVar[T any](t *testing.T, p *T, v T) {
	t.Helper()
	saved := *p
	*p = v
	t.Cleanup(func() { *p = saved })
}

// stubLookup resolves every name with fn.
func stubLookup(t *testing.T, fn func(ctx context.Context, network, host string) ([]net.IP, error)) {
	t.Helper()
	setVar(t, &lookupIP, fn)
}

// A name resolved slower than the connect bound, as when the first
// nameserver is dead and the second answers 5s later, is connected all the
// same: the lookup has its own bound.
func TestDialNameResolvedSlowerThanTheConnectBound(t *testing.T) {
	port := localListener(t)
	setVar(t, &dialTimeout, 200*time.Millisecond)
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		require.Equal(t, "node1", host)
		require.Equal(t, "ip", network)
		deadline, ok := ctx.Deadline()
		require.True(t, ok, "the lookup is bounded")
		require.Greater(t, time.Until(deadline), 10*time.Second, "by lookupTimeout")
		time.Sleep(400 * time.Millisecond)
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	})
	c, err := dialContext(context.Background(), "tcp", net.JoinHostPort("node1", port))
	require.NoError(t, err)
	_ = c.Close()
}

// A lookup never answering ends at lookupTimeout, as a timeout, which the
// retry of an unsent request knows.
func TestDialLookupBounded(t *testing.T) {
	setVar(t, &lookupTimeout, 100*time.Millisecond)
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		<-ctx.Done()
		return nil, &net.DNSError{Err: ctx.Err().Error(), Name: host, IsTimeout: true}
	})
	begin := time.Now()
	_, err := dialContext(context.Background(), "tcp", "node1:1215")
	require.Error(t, err)
	require.Less(t, time.Since(begin), 2*time.Second)
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout(), "%s", err)
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
}

// A name resolving to nothing fails as a host not found.
func TestDialNameWithNoAddress(t *testing.T) {
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		return nil, nil
	})
	_, err := dialContext(context.Background(), "tcp", "node1:1215")
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
	require.True(t, dnsErr.IsNotFound)
}

// A host given by its address is connected with no lookup, and the family
// of the network is asked of the lookup of a name.
func TestDialAddressAndFamily(t *testing.T) {
	port := localListener(t)
	var networks []string
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		networks = append(networks, network)
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	})
	c, err := dialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", port))
	require.NoError(t, err)
	_ = c.Close()
	require.Empty(t, networks, "an address is not looked up")

	c, err = dialContext(context.Background(), "tcp4", net.JoinHostPort("node1", port))
	require.NoError(t, err)
	_ = c.Close()
	require.Equal(t, []string{"ip4"}, networks)
}

// An address refusing the connection has the next one tried at once, not
// after attemptDelay.
func TestDialNextAddressAfterARefusal(t *testing.T) {
	port := localListener(t)
	setVar(t, &attemptDelay, 10*time.Second)
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.1")}, nil
	})
	begin := time.Now()
	c, err := dialContext(context.Background(), "tcp", net.JoinHostPort("node1", port))
	require.NoError(t, err)
	_ = c.Close()
	require.Less(t, time.Since(begin), 2*time.Second)
	require.Equal(t, "127.0.0.1:"+port, c.RemoteAddr().String())
}

// An address that does not answer, as one of a family the node can not
// reach, does not hold the connect to the next address past attemptDelay.
func TestDialNextAddressWhileOneHangs(t *testing.T) {
	port := localListener(t)
	setVar(t, &attemptDelay, 100*time.Millisecond)
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		// 192.0.2.1 is TEST-NET-1, routed nowhere: its connect hangs, or
		// fails at once where no route leads to it.
		return []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("127.0.0.1")}, nil
	})
	begin := time.Now()
	c, err := dialContext(context.Background(), "tcp", net.JoinHostPort("node1", port))
	require.NoError(t, err)
	_ = c.Close()
	require.Less(t, time.Since(begin), 2*time.Second)
}

// No address connecting returns the error of the first one.
func TestDialNoAddressConnects(t *testing.T) {
	port := localListener(t)
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3")}, nil
	})
	_, err := dialContext(context.Background(), "tcp", net.JoinHostPort("node1", port))
	require.Error(t, err)
	require.Contains(t, err.Error(), "127.0.0.2:"+port)
}

// The end of the context ends the connects.
func TestDialContextEnded(t *testing.T) {
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	_, err := dialContext(ctx, "tcp", "node1:1215")
	require.Error(t, err)
	require.Less(t, time.Since(begin), 2*time.Second)
}

// A request abandoned while its dial resolves the name leaves the dial
// running: the connection it makes is handed to the next request to the
// node, as the retry of a request aborted with no connection, which then
// does not wait for a lookup of its own.
func TestDialAbandonedServesTheNextRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	stubLookup(t, func(ctx context.Context, network, host string) ([]net.IP, error) {
		time.Sleep(700 * time.Millisecond)
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	})
	cli := &http.Client{Transport: NewTransport(nil)}
	t.Cleanup(cli.CloseIdleConnections)
	url := "http://" + net.JoinHostPort("node1", port)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	_, err = cli.Do(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// A lookup of its own would end at 1.2s, past its context.
	ctx, cancel = context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := cli.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
}
