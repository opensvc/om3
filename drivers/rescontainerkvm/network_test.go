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
	long, _ := nicNames("kvm/svc/vm1!container#1", 9)
	assert.LessOrEqual(t, len(long), 15, "an interface name")
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

func TestClaimNIC(t *testing.T) {
	saved := guestNetDir
	guestNetDir = t.TempDir()
	defer func() { guestNetDir = saved }()

	a := guestNIC{owner: "ns/svc/a!container#1", dev: "kv1n0", mac: "52:54:00:00:00:01"}
	require.NoError(t, claimNIC(a))
	require.NoError(t, claimNIC(a), "again, as a start after a crash")
	assert.True(t, ownsNIC(a))

	sameDev := guestNIC{owner: "ns/svc/b!container#1", dev: "kv1n0", mac: "52:54:00:00:00:02"}
	assert.ErrorContains(t, claimNIC(sameDev), "interface kv1n0 is held by ns/svc/a!container#1")
	assert.False(t, ownsNIC(sameDev), "unplugging it would cut the other guest")

	sameMAC := guestNIC{owner: "ns/svc/c!container#1", dev: "kv2n0", mac: "52:54:00:00:00:01"}
	assert.ErrorContains(t, claimNIC(sameMAC), "mac 52:54:00:00:00:01 is held by ns/svc/a!container#1")

	releaseNIC(sameDev)
	assert.True(t, ownsNIC(a), "released by who holds it only")
	releaseNIC(a)
	assert.NoError(t, claimNIC(sameDev))
}

func TestPartitionOf(t *testing.T) {
	for id, want := range map[string]string{
		"/opensvc.slice/opensvc-ns.rtqa.slice/opensvc-ns.rtqa-svc.kpg.slice":                                             "/opensvc/ns.rtqa/svc.kpg",
		`/opensvc.slice/opensvc-ns.rtqa.slice/opensvc-ns.rtqa-svc.k\x2dpg.slice`:                                         "/opensvc/ns.rtqa/svc.k-pg",
		`/opensvc.slice/opensvc-ns.a\x2db.slice/opensvc-ns.a\x2db-svc.c.slice/opensvc-ns.a\x2db-svc.c-container.1.slice`: "/opensvc/ns.a-b/svc.c/container.1",
		"/opensvc.slice/opensvc-svc.c.slice":                                                                             "/opensvc/svc.c",
	} {
		assert.Equal(t, want, partitionOf(id), id)
	}
}

func TestDefinitionWithPartition(t *testing.T) {
	const p = "/opensvc/ns.a/svc.b"
	want := "<domain>\n  <resource>\n    <partition>" + p + "</partition>\n  </resource>\n</domain>\n"

	b, err := definitionWithPartition([]byte("<domain>\n  <resource>\n    <partition>/machine</partition>\n  </resource>\n</domain>\n"), p)
	require.NoError(t, err)
	assert.Equal(t, want, string(b), "libvirt's, replaced")

	b, err = definitionWithPartition([]byte("<domain>\n</domain>\n"), p)
	require.NoError(t, err)
	assert.Equal(t, want, string(b), "none, added")

	b, err = definitionWithPartition([]byte("<domain>\n  <resource>\n    <fibrechannel appid='x'/>\n  </resource>\n</domain>\n"), p)
	require.NoError(t, err)
	assert.Contains(t, string(b), "<resource>\n    <partition>"+p+"</partition>\n    <fibrechannel", "beside what the section holds")

	_, err = definitionWithPartition([]byte("nothing"), p)
	assert.Error(t, err)
}

func TestDefinedMemory(t *testing.T) {
	assert.Equal(t, int64(1<<30), definedMemory([]byte("<memory unit='KiB'>1048576</memory>")))
	assert.Equal(t, int64(1<<30), definedMemory([]byte("<memory unit='GiB'>1</memory>")))
	assert.Equal(t, int64(1024), definedMemory([]byte("<memory>1</memory>")), "KiB by default")
	assert.Equal(t, int64(0), definedMemory([]byte("<domain/>")))
}

func TestDisplayAddress(t *testing.T) {
	for _, tc := range []struct {
		name, xml, network, address, err string
	}{
		{"socket", `<domain><devices><graphics type='vnc' socket='/var/lib/libvirt/qemu/d/vnc.sock'><listen type='socket' socket='/var/lib/libvirt/qemu/d/vnc.sock'/></graphics></devices></domain>`, "unix", "/var/lib/libvirt/qemu/d/vnc.sock", ""},
		{"port", `<domain><devices><graphics type='vnc' port='5901' autoport='yes' listen='0.0.0.0'/></devices></domain>`, "tcp", "127.0.0.1:5901", ""},
		{"vnc beside spice", `<domain><devices><graphics type='spice' port='5900'/><graphics type='vnc' port='5901'/></devices></domain>`, "tcp", "127.0.0.1:5901", ""},
		{"spice", `<domain><devices><graphics type='spice' port='5900'/></devices></domain>`, "", "", "a spice display"},
		{"none", `<domain><devices/></domain>`, "", "", "no display"},
		{"not started", `<domain><devices><graphics type='vnc' port='-1' autoport='yes'/></devices></domain>`, "", "", "no port"},
	} {
		network, address, err := displayAddress([]byte(tc.xml))
		if tc.err != "" {
			assert.ErrorContains(t, err, tc.err, tc.name)
			continue
		}
		assert.NoError(t, err, tc.name)
		assert.Equal(t, tc.network, network, tc.name)
		assert.Equal(t, tc.address, address, tc.name)
	}
}

func TestVMSpecGraphics(t *testing.T) {
	s := vmSpec{name: "vm1", vcpus: 1, memoryMB: 512, osFlag: "--osinfo", osValue: "x"}
	assert.Contains(t, strings.Join(s.args(), " "), "--graphics vnc,listen=socket", "no port of the node")
	s.extra = []string{"--graphics", "none"}
	assert.Equal(t, 1, strings.Count(strings.Join(s.args(), " "), "--graphics"), "root said otherwise")
}
