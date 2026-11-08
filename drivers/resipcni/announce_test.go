//go:build linux

package resipcni

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The addresses a start announces are the ones the CNI ADD result names, in
// the "ips" list of the 0.3.0 spec and later, or in the "ip4" and "ip6"
// sections before. A result not read names none, so nothing is announced.
func TestAddResultIPs(t *testing.T) {
	ips := addResultIPs([]byte(`{"cniVersion":"1.0.0","interfaces":[{"name":"eth0","sandbox":"/run/netns/x"}],"ips":[{"address":"10.140.0.61/24","gateway":"10.140.0.1","interface":0},{"address":"fd00::61/64","interface":0}]}`))
	assert.Equal(t, []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")}, ips)

	ips = addResultIPs([]byte(`{"cniVersion":"0.2.0","ip4":{"ip":"10.140.0.61/24"},"ip6":{"ip":"fd00::61/64"}}`))
	assert.Equal(t, []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")}, ips)

	assert.Empty(t, addResultIPs([]byte(`{"code":7,"msg":"failed"}`)))
	assert.Empty(t, addResultIPs([]byte(`not json`)))
	assert.Empty(t, addResultIPs(nil))
}
