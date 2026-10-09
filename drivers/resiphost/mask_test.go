package resiphost

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIPMaskIsNotCachedBeforeTheAddress is the regression test of an address
// drawn by a start, added with the mask a status read before the draw had
// cached: the mask of no address, which fell back to the ipv4 mask of the
// interface, and the address went up as a /24 the stop never found.
func TestIPMaskIsNotCachedBeforeTheAddress(t *testing.T) {
	r := &T{Netmask: "64", Dev: "lo"}
	r._ipaddr = nil
	r._ipaddrAge = 0
	r.Name = "not-a-resolvable-name.invalid"
	assert.Nil(t, r.ipmask(), "no address, no mask")
	assert.Nil(t, r._ipmask, "nothing cached")

	r._ipaddr = net.ParseIP("fd01:2345:6789:2902::5:9a")
	assert.Equal(t, 64, r.ipmaskOnes())
}

// TestDefaultMaskKeepsToTheFamily pins that the mask guessed from the
// interface is the mask of an address of the same family.
func TestDefaultMaskKeepsToTheFamily(t *testing.T) {
	r := &T{Dev: "lo"}
	m, err := r.defaultMask(32)
	if assert.NoError(t, err) {
		ones, bits := m.Size()
		assert.Equal(t, 32, bits)
		assert.Equal(t, 8, ones)
	}
	if m, err := r.defaultMask(128); err == nil {
		_, bits := m.Size()
		assert.Equal(t, 128, bits, "lo has ::1/128 where ipv6 is on")
	}
}
