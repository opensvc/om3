//go:build linux

package resipcni

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The addresses a start announces are the ones the CNI ADD result names on
// the card the plugin added, in the "ips" list of the 0.3.0 spec and later,
// or in the "ip4" and "ip6" sections before. An address on another card is
// on another link, and is not announced over this one. A result not read
// names none, so nothing is announced.
func TestAddResultIPs(t *testing.T) {
	ips := addResultIPs([]byte(`{"cniVersion":"1.0.0","interfaces":[{"name":"eth0","sandbox":"/run/netns/x"}],"ips":[{"address":"10.140.0.61/24","gateway":"10.140.0.1","interface":0},{"address":"fd00::61/64","interface":0}]}`), "eth0")
	assert.Equal(t, []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")}, ips)

	ips = addResultIPs([]byte(`{"cniVersion":"0.2.0","ip4":{"ip":"10.140.0.61/24"},"ip6":{"ip":"fd00::61/64"}}`), "eth0")
	assert.Equal(t, []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")}, ips)

	ips = addResultIPs([]byte(`{"cniVersion":"1.0.0","interfaces":[{"name":"veth1a2b"},{"name":"eth0","sandbox":"/run/netns/x"},{"name":"net1","sandbox":"/run/netns/x"}],"ips":[{"address":"10.140.0.61/24","interface":1},{"address":"192.168.9.61/24","interface":2},{"address":"10.99.0.1/24","interface":0},{"address":"10.140.0.62/24","interface":7},{"address":"fd00::61/64"}]}`), "eth0")
	assert.Equal(t, []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")}, ips, "the addresses on eth0, or naming no card")

	assert.Empty(t, addResultIPs([]byte(`{"code":7,"msg":"failed"}`), "eth0"))
	assert.Empty(t, addResultIPs([]byte(`not json`), "eth0"))
	assert.Empty(t, addResultIPs(nil, "eth0"))
}
