//go:build linux

package network

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// procNetFiles are the socket tables of the network namespace of the node, by
// protocol, replaced by the tests.
var procNetFiles = map[string][]string{
	"tcp": {"/proc/net/tcp", "/proc/net/tcp6"},
	"udp": {"/proc/net/udp", "/proc/net/udp6"},
}

// boundStates are the socket states holding a port for the connections to
// come: a tcp socket listening, an udp one bound and not connected.
var boundStates = map[string]string{
	"tcp": "0A",
	"udp": "07",
}

// PublishesBound returns the publishes to addr whose port of the node a
// socket of the node holds too: the forwarding takes the connections that
// process waits for. A socket of a loopback address alone is no conflict, its
// connections not being forwarded, nor a socket of the other family than
// addr, which the forwarding of that family does not take the connections
// of.
//
// Nothing keeps a process of the node from binding a published port, the one
// drawn for a publish naming none included: the status says it.
func PublishesBound(addr net.IP, publishes []Publish) ([]Publish, error) {
	v4 := addr.To4() != nil
	var bound []Publish
	cache := make(map[string][]boundSocket)
	for _, p := range publishes {
		if p.Random() {
			continue
		}
		sockets, ok := cache[p.Proto]
		if !ok {
			var err error
			if sockets, err = boundSockets(p.Proto); err != nil {
				return nil, err
			}
			cache[p.Proto] = sockets
		}
		for _, s := range sockets {
			if s.port != p.HostPort || s.addr.IsLoopback() || !receivesFamily(s.addr, v4) {
				continue
			}
			if p.HostAddr == nil || s.addr.IsUnspecified() || s.addr.Equal(p.HostAddr) {
				bound = append(bound, p)
				break
			}
		}
	}
	return bound, nil
}

// receivesFamily says a socket of the address a receives the connections of
// the ipv4 family, or else of the ipv6 one.
//
// An ipv4 socket, or an ipv6 one of an ipv4 mapped address, receives the
// ipv4 connections only. The ipv6 wildcard receives the ipv4 connections too,
// its sockets being dual stack by default on linux: a v6only one does not,
// which the socket tables do not say, so it is taken for one that does.
func receivesFamily(a net.IP, v4 bool) bool {
	if a.To4() != nil {
		return v4
	}
	return !v4 || a.Equal(net.IPv6unspecified)
}

type boundSocket struct {
	addr net.IP
	port int
}

// boundSockets reads the sockets of a protocol holding a port.
func boundSockets(proto string) ([]boundSocket, error) {
	var l []boundSocket
	for _, file := range procNetFiles[proto] {
		f, err := os.Open(file)
		if os.IsNotExist(err) {
			// No ipv6 on the node.
			continue
		}
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Scan() // the header
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 || fields[3] != boundStates[proto] {
				continue
			}
			s, err := parseProcNetAddr(fields[1])
			if err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			l = append(l, s)
		}
		err = scanner.Err()
		_ = f.Close()
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

// parseProcNetAddr reads an address of a socket table, as 0100007F:1F90: the
// address in hexadecimal, by 32 bits words in the byte order of the host,
// and the port.
func parseProcNetAddr(s string) (boundSocket, error) {
	addrHex, portHex, ok := strings.Cut(s, ":")
	if !ok {
		return boundSocket{}, fmt.Errorf("invalid socket address %q", s)
	}
	b, err := hex.DecodeString(addrHex)
	if err != nil || (len(b) != net.IPv4len && len(b) != net.IPv6len) {
		return boundSocket{}, fmt.Errorf("invalid socket address %q", s)
	}
	for i := 0; i < len(b); i += 4 {
		// The word as the kernel printed it, put back in memory the way
		// the host stores it, which is the address in network order.
		binary.NativeEndian.PutUint32(b[i:i+4], binary.BigEndian.Uint32(b[i:i+4]))
	}
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return boundSocket{}, fmt.Errorf("invalid socket address %q", s)
	}
	return boundSocket{addr: net.IP(b), port: int(port)}, nil
}
