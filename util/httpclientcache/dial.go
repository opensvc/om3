package httpclientcache

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/opensvc/om3/v3/util/stagger"
)

var (
	// lookupTimeout bounds the resolution of the name of a host, apart
	// from its connect: the resolver waits 5s on a nameserver that does
	// not answer before it asks the next one, so a connect bound covering
	// the lookup too would end with the lookup of a node whose first
	// nameserver is dead, at the instant the second would answer. 15s is
	// one pass over three nameservers.
	lookupTimeout = 15 * time.Second

	// attemptDelay is how long the connect to an address runs before the
	// connect to the next address of the name starts too, as an address of
	// a family the node can not reach, an IPv6 one on a network routing
	// IPv4 only, would otherwise hold the connect for dialTimeout.
	attemptDelay = 300 * time.Millisecond

	// lookupIP resolves the name of a host.
	lookupIP = net.DefaultResolver.LookupIP
)

// dialContext connects to address. A host given by its name is resolved
// within lookupTimeout, then each of its addresses is given dialTimeout to
// connect, in the order of the resolver, the next one starting when the
// previous one failed or ran attemptDelay. A host given by its address is
// connected within dialTimeout, with no lookup.
func dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d := &net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return d.DialContext(ctx, network, address)
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return d.DialContext(ctx, network, address)
	}
	ips, err := lookup(ctx, network, host)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Err: err}
	}
	addrs := make([]string, len(ips))
	for i, ip := range ips {
		addrs[i] = net.JoinHostPort(ip.String(), port)
	}
	return dialAddrs(ctx, d, network, addrs)
}

// lookup returns the addresses of host for network, resolved within
// lookupTimeout.
func lookup(ctx context.Context, network, host string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	ipNetwork := "ip"
	switch network {
	case "tcp4":
		ipNetwork = "ip4"
	case "tcp6":
		ipNetwork = "ip6"
	}
	ips, err := lookupIP(ctx, ipNetwork, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return ips, nil
}

// dialAddrs connects to the first of addrs to accept the connection. The
// connect to an address starts when the one to the previous address failed
// or ran attemptDelay, and the connects still running are abandoned at the
// first established. It returns the error of the first address when none
// connects.
func dialAddrs(ctx context.Context, d *net.Dialer, network string, addrs []string) (net.Conn, error) {
	attempts := make([]stagger.Attempt, len(addrs))
	for i, addr := range addrs {
		attempts[i] = func(ctx context.Context) (net.Conn, error) {
			return d.DialContext(ctx, network, addr)
		}
	}
	return stagger.Dial(ctx, attemptDelay, attempts)
}
