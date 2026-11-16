//go:build linux

package network

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tables below are written as a little endian host prints them.
const (
	procNetTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:6369 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 2 1 0000000000000000 100 0 0 10 0
   2: 0B002D0A:01BB 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 3 1 0000000000000000 100 0 0 10 0
   3: 0B002D0A:0016 0C002D0A:D431 01 00000000:00000000 00:00000000 00000000     0        0 4 1 0000000000000000 100 0 0 10 0
   4: 00000000:1F94 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 7 1 0000000000000000 100 0 0 10 0
`
	procNetTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:1F91 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 5 1 0000000000000000 100 0 0 10 0
   1: 000000FD000000000000000001000000:1F92 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 8 1 0000000000000000 100 0 0 10 0
   2: 0000000000000000FFFF00000B002D0A:1F93 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 9 1 0000000000000000 100 0 0 10 0
`
	procNetUDP = `   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
  0: 00000000:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000     0        0 6 2 0000000000000000 0
`
)

func withProcNet(t *testing.T) {
	t.Helper()
	if binary.NativeEndian.Uint16([]byte{1, 0}) != 1 {
		t.Skip("the tables are written as a little endian host prints them")
	}
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0600))
		return p
	}
	save := procNetFiles
	procNetFiles = map[string][]string{
		"tcp": {write("tcp", procNetTCP), write("tcp6", procNetTCP6)},
		"udp": {write("udp", procNetUDP), filepath.Join(dir, "absent")},
	}
	t.Cleanup(func() { procNetFiles = save })
}

func TestPublishesBound(t *testing.T) {
	withProcNet(t)
	publishes := mustPublishes(t,
		"25449:80",           // 0x6369 listened on every address: bound
		"8080:80/tcp",        // 0x1F90 listened on 127.0.0.1 only: not forwarded, no conflict
		"10.45.0.11:443:443", // 0x01BB listened on 10.45.0.11: bound
		"10.45.0.12:22:22",   // 0x0016 a connection, not a listener
		"8081:81",            // 0x1F91 listened on every ipv6 address: bound
		"53:53/udp",          // 0x0035 udp bound: bound
		"53:53/tcp",          // not listened in tcp
		"80",                 // a port to draw is not one yet
	)
	bound, err := PublishesBound(net.ParseIP("10.22.0.5"), publishes)
	require.NoError(t, err)
	got := make([]string, len(bound))
	for i, p := range bound {
		got[i] = p.String()
	}
	assert.Equal(t, []string{"25449:80/tcp", "10.45.0.11:443:443/tcp", "8081:81/tcp", "53:53/udp"}, got)
}

func TestParseProcNetAddr(t *testing.T) {
	if binary.NativeEndian.Uint16([]byte{1, 0}) != 1 {
		t.Skip("written as a little endian host prints them")
	}
	s, err := parseProcNetAddr("0100007F:1F90")
	require.NoError(t, err)
	assert.True(t, s.addr.Equal(net.ParseIP("127.0.0.1")))
	assert.Equal(t, 8080, s.port)

	s, err = parseProcNetAddr("0000000000000000FFFF00000B002D0A:01BB")
	require.NoError(t, err)
	assert.True(t, s.addr.Equal(net.ParseIP("10.45.0.11")), s.addr.String())

	for _, bad := range []string{"", "0100007F", "zz:1F90", "0100007F:zz", "01007F:1F90"} {
		_, err := parseProcNetAddr(bad)
		assert.Error(t, err, bad)
	}
}

// TestPublishesBoundReadsTheNode reads the socket tables of the machine
// running the test, a listener of its own among them.
func TestPublishesBoundReadsTheNode(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	bound, err := PublishesBound(net.ParseIP("10.22.0.5"), []Publish{{HostPort: port, Port: 80, Proto: "tcp"}})
	require.NoError(t, err)
	assert.Len(t, bound, 1)
}

// TestPublishesBoundComparesTheFamilyOfTheAddress pins that a socket is a
// conflict only for the publishes of the family it receives the connections
// of: the ipv6 wildcard, dual stack by default, receives both.
func TestPublishesBoundComparesTheFamilyOfTheAddress(t *testing.T) {
	withProcNet(t)
	for _, tc := range []struct {
		name string
		port string
		v4   bool
		v6   bool
	}{
		{name: "ipv6 wildcard, dual stack", port: "8081:81", v4: true, v6: true},
		{name: "an ipv6 address", port: "8082:82", v4: false, v6: true},
		{name: "an ipv4 mapped address", port: "8083:83", v4: true, v6: false},
		{name: "ipv4 wildcard", port: "8084:84", v4: true, v6: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publishes := mustPublishes(t, tc.port)
			bound, err := PublishesBound(net.ParseIP("10.22.0.5"), publishes)
			require.NoError(t, err)
			assert.Equal(t, tc.v4, len(bound) == 1, "ipv4 publish")
			bound, err = PublishesBound(net.ParseIP("fd01::5"), publishes)
			require.NoError(t, err)
			assert.Equal(t, tc.v6, len(bound) == 1, "ipv6 publish")
		})
	}
}
