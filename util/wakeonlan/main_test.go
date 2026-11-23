package wakeonlan

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMAC(t *testing.T) {
	for _, s := range []string{"52:54:00:aa:bb:cc", "52-54-00-AA-BB-CC", "525400aabbcc"} {
		mac, err := ParseMAC(s)
		require.NoError(t, err, s)
		assert.Equal(t, "52:54:00:aa:bb:cc", mac.String(), s)
	}
	for _, s := range []string{"", "52:54:00:aa:bb", "52:54:00:aa:bb:zz", "00:00:00:00:fe:80:00:00:00:00:00:00:02:00:5e:10:00:00:00:01"} {
		_, err := ParseMAC(s)
		assert.Error(t, err, s)
	}
}

func TestParseBroadcast(t *testing.T) {
	ip, err := ParseBroadcast("10.0.0.255")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.255", ip.String())
	for _, s := range []string{"", "10.0.0", "10.0.0.256", "ff02::1", "host.example"} {
		_, err := ParseBroadcast(s)
		assert.Error(t, err, s)
	}
}

func TestValidatePort(t *testing.T) {
	assert.NoError(t, ValidatePort(7))
	assert.NoError(t, ValidatePort(DefaultPort))
	assert.Error(t, ValidatePort(0))
	assert.Error(t, ValidatePort(65536))
}

func TestMagicPacket(t *testing.T) {
	mac, err := ParseMAC("52:54:00:aa:bb:cc")
	require.NoError(t, err)
	b := MagicPacket(mac)
	require.Len(t, b, 102)
	assert.Equal(t, bytes.Repeat([]byte{0xFF}, 6), b[:6])
	for i := 0; i < 16; i++ {
		assert.Equal(t, []byte(mac), b[6+i*6:12+i*6], "repetition %d", i)
	}
}

func TestSend(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	port := conn.LocalAddr().(*net.UDPAddr).Port

	require.NoError(t, Send("52:54:00:aa:bb:cc", "127.0.0.1", port))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	buf := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buf)
	require.NoError(t, err)
	mac, _ := ParseMAC("52:54:00:aa:bb:cc")
	assert.Equal(t, MagicPacket(mac), buf[:n])
}

func TestSendInvalid(t *testing.T) {
	assert.Error(t, Send("bad", "127.0.0.1", 9))
	assert.Error(t, Send("52:54:00:aa:bb:cc", "bad", 9))
	assert.Error(t, Send("52:54:00:aa:bb:cc", "127.0.0.1", 0))
}
