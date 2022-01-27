package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	mac0, _ = net.ParseMAC("0a:58:01:02:03:04")
)

func TestMACFromIP4(t *testing.T) {
	tests := []struct {
		ip  net.IP
		mac net.HardwareAddr
	}{
		{
			ip:  net.ParseIP("1.2.3.4"),
			mac: mac0,
		},
	}
	for _, test := range tests {
		t.Run(test.ip.String(), func(t *testing.T) {
			mac, err := MACFromIP4(test.ip)
			t.Run("no error", func(t *testing.T) {
				assert.Nil(t, err)
			})
			t.Run("generated mac is correct", func(t *testing.T) {
				assert.Equal(t, test.mac, mac)
			})
		})
	}
}
