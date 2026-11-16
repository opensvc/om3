package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePublish(t *testing.T) {
	cases := map[string]Publish{
		"25489:80":               {HostPort: 25489, Port: 80, Proto: "tcp"},
		"25489:80/tcp":           {HostPort: 25489, Port: 80, Proto: "tcp"},
		"53:5353/udp":            {HostPort: 53, Port: 5353, Proto: "udp"},
		"10.0.0.5:8443:443":      {HostAddr: net.ParseIP("10.0.0.5").To4(), HostPort: 8443, Port: 443, Proto: "tcp"},
		"[fd00::5]:8443:443/udp": {HostAddr: net.ParseIP("fd00::5"), HostPort: 8443, Port: 443, Proto: "udp"},
		"80":                     {Port: 80, Proto: "tcp"},
		"80/udp":                 {Port: 80, Proto: "udp"},
		"10.0.0.5::80":           {HostAddr: net.ParseIP("10.0.0.5").To4(), Port: 80, Proto: "tcp"},
		"[fd00::5]::80/udp":      {HostAddr: net.ParseIP("fd00::5"), Port: 80, Proto: "udp"},
	}
	for s, want := range cases {
		t.Run(s, func(t *testing.T) {
			got, err := ParsePublish(s)
			require.NoError(t, err)
			assert.Equal(t, want, got)
			again, err := ParsePublish(got.String())
			require.NoError(t, err)
			assert.Equal(t, got, again, "String() reads back")
		})
	}
}

func TestParsePublishRefuses(t *testing.T) {
	for _, s := range []string{
		"",
		":80",
		"80:",
		"10.0.0.5::",
		"[fd00::5]:80",
		"[fd00::5]:1:2:3",
		"25489:80/sctp",
		"0:80",
		"25489:65536",
		"http:80",
		"a.b.c.d:1:2",
		"fd00::5:8443:443",
		"[10.0.0.5]:8443:443",
		"[fd00::5:8443:443",
		"1:2:3:4",
	} {
		t.Run(s, func(t *testing.T) {
			_, err := ParsePublish(s)
			assert.Error(t, err)
		})
	}
}

func TestParsePublishesRefusesAPortClaimedTwice(t *testing.T) {
	_, err := ParsePublishes([]string{"8080:80", "10.0.0.5:8080:81"})
	assert.ErrorContains(t, err, "claim the same port")

	_, err = ParsePublishes([]string{"8080:80", "8080:80/udp", "10.0.0.5:8081:81", "10.0.0.6:8081:81"})
	assert.NoError(t, err, "other protocol or other address of the node")
}

func TestParsePublishesRandomHostPorts(t *testing.T) {
	_, err := ParsePublishes([]string{"80", "80/tcp"})
	assert.ErrorContains(t, err, "declared twice")

	_, err = ParsePublishes([]string{"80", "80/udp", "10.0.0.5::80", "8080:80", "81"})
	assert.NoError(t, err, "a port to draw claims no port of the node yet")
}

func TestPublishDeclared(t *testing.T) {
	p := Publish{HostPort: 30123, Port: 80, Proto: "tcp", Auto: true}
	assert.Equal(t, "80/tcp", p.Declared().String())
	fixed := Publish{HostPort: 25489, Port: 80, Proto: "tcp"}
	assert.Equal(t, fixed, fixed.Declared())
}

func TestValidatePublish(t *testing.T) {
	assert.NoError(t, ValidatePublish(""))
	assert.NoError(t, ValidatePublish("25489:80/tcp 192.168.1.10:8443:443/tcp"))
	assert.Error(t, ValidatePublish("25489:80/tcp nope"))
}
