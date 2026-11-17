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

// procNetTables are the socket tables of the network namespace of the node,
// ipv4 and ipv6.
type procNetTables struct {
	v4, v6 string
}

// procNetFiles are the socket tables of the network namespace of the node, by
// protocol, replaced by the tests.
var procNetFiles = map[string]procNetTables{
	"tcp": {v4: "/proc/net/tcp", v6: "/proc/net/tcp6"},
	"udp": {v4: "/proc/net/udp", v6: "/proc/net/udp6"},
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
// of, a v6only ipv6 wildcard socket for an ipv4 addr included.
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
			if s.port != p.HostPort || s.addr.IsLoopback() || !s.receivesFamily(v4) {
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

// receivesFamily says the socket receives the connections of the ipv4
// family, or else of the ipv6 one.
//
// An ipv4 socket, or an ipv6 one of an ipv4 mapped address, receives the
// ipv4 connections only. The ipv6 wildcard receives the ipv4 connections too
// unless it is v6only. The socket tables of /proc not saying the v6only
// option, a socket read from them is taken for a dual stack one, the default
// on linux.
func (s boundSocket) receivesFamily(v4 bool) bool {
	if s.addr.To4() != nil {
		return v4
	}
	return !v4 || (s.addr.Equal(net.IPv6unspecified) && !s.v6only)
}

type boundSocket struct {
	addr   net.IP
	port   int
	v6only bool
}

// boundSockets reads the sockets of a protocol holding a port: the ipv4 ones
// from the socket table of /proc, the ipv6 ones from the sock_diag interface
// of the kernel, which says their v6only option, or from the socket table of
// /proc when the kernel does not answer.
func boundSockets(proto string) ([]boundSocket, error) {
	tables := procNetFiles[proto]
	l, err := readProcNet(tables.v4, proto)
	if err != nil {
		return nil, err
	}
	l6, err := diagV6Sockets(proto)
	if err != nil {
		if l6, err = readProcNet(tables.v6, proto); err != nil {
			return nil, err
		}
	}
	return append(l, l6...), nil
}

// readProcNet reads the sockets of a protocol holding a port from a socket
// table of /proc.
func readProcNet(file, proto string) ([]boundSocket, error) {
	f, err := os.Open(file)
	if os.IsNotExist(err) {
		// No ipv6 on the node.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var l []boundSocket
	scanner := bufio.NewScanner(f)
	scanner.Scan() // the header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[3] != boundStates[proto] {
			continue
		}
		s, err := parseProcNetAddr(fields[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		l = append(l, s)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
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
