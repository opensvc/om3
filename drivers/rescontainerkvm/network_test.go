package rescontainerkvm

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	require.NoError(t, err)
	return n
}

func TestNicNames(t *testing.T) {
	dev, mac := nicNames("kvm/svc/vm1!container#1", 0)
	assert.LessOrEqual(t, len(dev), 15, "an interface name")
	_, err := net.ParseMAC(mac)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(mac, "52:54:"))
	dev2, mac2 := nicNames("kvm/svc/vm1!container#1", 1)
	assert.NotEqual(t, dev, dev2)
	assert.NotEqual(t, mac, mac2)
	dev3, mac3 := nicNames("kvm/svc/vm2!container#1", 0)
	assert.NotEqual(t, dev, dev3)
	assert.NotEqual(t, mac, mac3)
}

func TestLinkLocal(t *testing.T) {
	ip, err := linkLocal("52:54:00:a0:00:0a")
	require.NoError(t, err)
	assert.Equal(t, "fe80::5054:ff:fea0:a", ip.String())
}

func TestFilterRulesIPv4(t *testing.T) {
	s, err := filterRules(guestNIC{dev: "kv1n0", mac: "52:54:00:a0:00:0a", ip: net.ParseIP("10.100.0.5")})
	require.NoError(t, err)
	assert.Contains(t, s, `hook ingress device "kv1n0"`)
	assert.Contains(t, s, "policy drop")
	assert.Contains(t, s, "ether saddr != 52:54:00:a0:00:0a drop")
	assert.Contains(t, s, "arp saddr ip { 10.100.0.5, 0.0.0.0 } accept")
	assert.Contains(t, s, "ether type ip ip saddr 10.100.0.5 accept")
	assert.Contains(t, s, "ip6 saddr { fe80::5054:ff:fea0:a, :: } accept")
	assert.Less(t, strings.Index(s, "udp sport 67 drop"), strings.Index(s, "ip saddr 10.100.0.5 accept"), "a guest is no dhcp server, with its address either")
}

func TestFilterRulesIPv6(t *testing.T) {
	s, err := filterRules(guestNIC{dev: "kv1n0", mac: "52:54:00:a0:00:0a", ip: net.ParseIP("fd01::5")})
	require.NoError(t, err)
	assert.NotContains(t, s, "arp", "no ipv4 on an ipv6 network")
	assert.NotContains(t, s, "ip saddr")
	assert.Contains(t, s, "ip6 saddr { fe80::5054:ff:fea0:a, ::, fd01::5 } accept")
	assert.Contains(t, s, "@th,64,128 { 0xfe800000000000005054", "the targets of its advertisements")
	assert.Contains(t, s, "0xfd010000000000000000000000000005 }")
	assert.Less(t, strings.Index(s, "nd-router-advert"), strings.Index(s, "ip6 saddr {"), "a guest is no router, with its address either")
}

func TestNetworkConfig(t *testing.T) {
	s := networkConfig([]guestNIC{
		{mac: "52:54:00:00:00:01", ip: net.ParseIP("10.100.0.5")},
		{mac: "52:54:00:00:00:02", ip: net.ParseIP("fd01::5"), whole: mustCIDR(t, "fd01::/64")},
		{mac: "52:54:00:00:00:03", ip: net.ParseIP("10.77.0.200"), static: true, prefix: 24, gateway: net.ParseIP("10.77.0.1"), nameservers: []string{"1.1.1.1"}},
	})
	assert.Equal(t, `version: 2
ethernets:
  nic0:
    match:
      macaddress: "52:54:00:00:00:01"
    dhcp4: true
    dhcp6: false
    link-local: []
  nic1:
    match:
      macaddress: "52:54:00:00:00:02"
    dhcp4: false
    dhcp6: true
    ipv6-address-generation: eui64
    accept-ra: true
    routes:
      - to: fd01::/64
        via: "fe80::1"
        on-link: true
  nic2:
    match:
      macaddress: "52:54:00:00:00:03"
    dhcp4: false
    dhcp6: false
    link-local: []
    addresses:
      - 10.77.0.200/24
    routes:
      - to: default
        via: 10.77.0.1
    nameservers:
      addresses: [1.1.1.1]
`, s)
}

func TestDnsmasqArgs(t *testing.T) {
	n := guestNIC{
		master:  "obr_b3",
		ip:      net.ParseIP("10.100.1.5"),
		subnet:  mustCIDR(t, "10.100.1.0/24"),
		whole:   mustCIDR(t, "10.100.0.0/22"),
		gateway: net.ParseIP("10.100.1.1"),
	}
	l := dnsmasqArgs("/run/x", n)
	assert.Contains(t, l, "--interface=obr_b3")
	assert.Contains(t, l, "--dhcp-range=10.100.1.0,static,255.255.255.0,infinite")
	assert.Contains(t, l, "--dhcp-option=121,10.100.0.0/22,10.100.1.1,0.0.0.0/0,10.100.1.1")
	assert.NotContains(t, l, "--enable-ra")

	n = guestNIC{master: "obr_b5", ip: net.ParseIP("fd01::5"), subnet: mustCIDR(t, "fd01::/64"), nameservers: []string{"fd02::1"}}
	l = dnsmasqArgs("/run/x", n)
	assert.Contains(t, l, "--dhcp-range=fd01::,static,64,infinite")
	assert.Contains(t, l, "--enable-ra")
	assert.Contains(t, l, "--dhcp-option=option6:dns-server,[fd02::1]")
}

func TestDhcpHost(t *testing.T) {
	assert.Equal(t, "52:54:00:00:00:01,10.0.0.5\n", dhcpHost(guestNIC{mac: "52:54:00:00:00:01", ip: net.ParseIP("10.0.0.5")}))
	assert.Equal(t, "52:54:00:00:00:01,[fd01::5]\n", dhcpHost(guestNIC{mac: "52:54:00:00:00:01", ip: net.ParseIP("fd01::5")}))
}

func TestSpecNetworkError(t *testing.T) {
	assert.NoError(t, (&T{Network: []string{"a", "b"}}).specNetworkError())
	assert.ErrorContains(t, (&T{Network: []string{"a", "a"}}).specNetworkError(), "twice")
}

func TestVMSpecArgsOnNetworks(t *testing.T) {
	s := vmSpec{
		name: "vm1", vcpus: 1, memoryMB: 1024, osFlag: "--osinfo", osValue: "x",
		bridges:  []string{"br-prd"},
		nics:     []guestNIC{{dev: "kv1n0", mac: "52:54:00:00:00:01"}},
		userData: "/tmp/ud",
		netData:  "/tmp/nd",
	}
	args := strings.Join(s.args(), " ")
	assert.Contains(t, args, "--network type=ethernet,target.dev=kv1n0,mac=52:54:00:00:00:01,model=virtio")
	assert.Contains(t, args, "--xml ./devices/interface[2]/target/@managed=no", "after the bridge")
	assert.Contains(t, args, "--cloud-init user-data=/tmp/ud,network-config=/tmp/nd")
}
