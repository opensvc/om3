//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resip"
)

// inNodeNetns runs fn in a new network namespace standing for the one of
// the node, on a thread locked to it, and skips the test when not root. The
// commands the resource runs are forked from that thread, and run in it.
func inNodeNetns(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("needs root to create a network namespace")
	}
	for _, name := range []string{"ip", "nsenter", "sleep"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("needs %s", name)
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		// The thread is left locked: it ends with the goroutine, and
		// with it the namespace.
		node, err := netns.New()
		if !assert.NoError(t, err) {
			return
		}
		defer node.Close()
		// The bridges of the test namespace are not in the sysfs of the
		// one the test was started in.
		prev := isLinuxBridge
		isLinuxBridge = func(dev string) bool {
			link, err := netlink.LinkByName(dev)
			return err == nil && link.Type() == "bridge"
		}
		defer func() { isLinuxBridge = prev }()
		fn(t)
	}()
	<-done
}

// fakeContainer is the resource the netns keyword names: a process in a
// network namespace of its own.
type fakeContainer struct {
	resource.Driver
	pid int
}

func (t fakeContainer) PID(context.Context) int { return t.pid }
func (t fakeContainer) NetNSPath(context.Context) (string, error) {
	if t.pid == 0 {
		return "", nil
	}
	return fmt.Sprintf("/proc/%d/ns/net", t.pid), nil
}

// containerObject is an object whose container#<n> resources are the fake
// containers.
type containerObject struct {
	fakeObject
	containers map[string]fakeContainer
}

func (t containerObject) ResourceByID(rid string) resource.Driver {
	if c, ok := t.containers[rid]; ok {
		return c
	}
	return nil
}

// startContainer starts a process in a new network namespace, killed at the
// end of the test.
func startContainer(t *testing.T) fakeContainer {
	t.Helper()
	cmd := exec.Command("sleep", "600")
	cmd.SysProcAttr = &syscall.SysProcAttr{Unshareflags: syscall.CLONE_NEWNET}
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return fakeContainer{pid: cmd.Process.Pid}
}

func addBridge(t *testing.T, name string) {
	t.Helper()
	la := netlink.NewLinkAttrs()
	la.Name = name
	br := &netlink.Bridge{LinkAttrs: la}
	require.NoError(t, netlink.LinkAdd(br))
	require.NoError(t, netlink.LinkSetUp(br))
}

// guestState returns the veth links of the namespace of the container, their
// ipv4 addresses, its ipv4 default routes and the number of
// the policy routing rules om adds.
func guestState(t *testing.T, c fakeContainer) (links []netlink.Link, addrs []string, defaults []string, rules int) {
	t.Helper()
	path, _ := c.NetNSPath(context.Background())
	guest, err := ns.GetNS(path)
	require.NoError(t, err)
	defer guest.Close()
	require.NoError(t, guest.Do(func(ns.NetNS) error {
		all, err := netlink.LinkList()
		if err != nil {
			return err
		}
		for _, link := range all {
			if link.Type() != "veth" {
				// The loopback, and the fallback devices of the
				// tunnel modules the node has loaded.
				continue
			}
			links = append(links, link)
			l, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				return err
			}
			for _, a := range l {
				addrs = append(addrs, a.IPNet.String())
			}
		}
		routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
		if err != nil {
			return err
		}
		for _, r := range routes {
			if r.Dst == nil || r.Dst.IP.IsUnspecified() {
				defaults = append(defaults, r.Gw.String())
			}
		}
		l, err := netlink.RuleList(netlink.FAMILY_V4)
		if err != nil {
			return err
		}
		for _, r := range l {
			if r.Priority == resip.SourceRulePriority {
				rules++
			}
		}
		return nil
	}))
	return
}

func statusErrors(r *T) string {
	l := make([]string, 0)
	for _, e := range r.StatusLog().Entries() {
		l = append(l, e.String())
	}
	return strings.Join(l, "\n")
}

// With no name, the resource joins the segment and stops there: its
// interface is up in the namespace, with the stable hardware address and no
// address, its peer a port of the bridge with ipv6 disabled. It is up then,
// down when the peer is off the bridge, which a start plugs back, and its
// stop deletes the pair. The bridge missing is said in so many words.
func TestL2OnlyInNamespaces(t *testing.T) {
	inNodeNetns(t, func(t *testing.T) {
		addBridge(t, "obr_seg")
		c := startContainer(t)
		obj := containerObject{containers: map[string]fakeContainer{"container#0": c}}
		r := newL2Resource(t, "ip#1", obj)
		require.NoError(t, r.Configure())
		mac := stableMAC(testObjectID.String(), "ip#1", "").String()
		ctx := actionrollback.NewContext(context.Background())
		status := func() status.T {
			r.StatusLog().Reset()
			return r.Status(ctx)
		}

		assert.Equal(t, "down", status().String())
		require.NoError(t, r.Start(ctx))

		links, addrs, defaults, rules := guestState(t, c)
		require.Len(t, links, 1)
		guest := links[0].Attrs()
		assert.Equal(t, "eth0", guest.Name)
		assert.Equal(t, mac, guest.HardwareAddr.String())
		assert.NotZero(t, guest.Flags&net.FlagUp, "up")
		assert.Empty(t, addrs, "no address")
		assert.Empty(t, defaults, "no route")
		assert.Zero(t, rules, "no rule")

		hostDev := formatHostDevName("eth0", c.pid)
		peer, err := netlink.LinkByName(hostDev)
		require.NoError(t, err)
		br, err := netlink.LinkByName("obr_seg")
		require.NoError(t, err)
		assert.Equal(t, br.Attrs().Index, peer.Attrs().MasterIndex, "a port of the bridge")
		if b, err := os.ReadFile("/proc/sys/net/ipv6/conf/" + hostDev + "/disable_ipv6"); err == nil {
			assert.Equal(t, "1", strings.TrimSpace(string(b)), "no ipv6 on the end of the node")
		}
		assert.Equal(t, "up", status().String(), statusErrors(r))
		assert.Equal(t, mac, r.StatusInfo(ctx)["macaddr"])
		assert.NotContains(t, r.StatusInfo(ctx), "ipaddr")

		// An error of the configuration is no ConfigError while the
		// namespace holds the interface.
		r.netErr = errors.New("misfit")
		assert.NoError(t, r.ConfigError(ctx))
		r.netErr = nil

		require.NoError(t, r.Start(ctx), "a second start changes nothing")
		links, _, _, _ = guestState(t, c)
		assert.Len(t, links, 1)

		require.NoError(t, netlink.LinkSetNoMaster(peer))
		assert.Equal(t, "down", status().String())
		assert.Contains(t, statusErrors(r), "is not a port of obr_seg, the bridge of network seg")
		require.NoError(t, r.Start(ctx), "plugged back")
		assert.Equal(t, "up", status().String(), statusErrors(r))

		require.NoError(t, r.Stop(ctx))
		links, _, _, _ = guestState(t, c)
		assert.Empty(t, links)
		_, err = netlink.LinkByName(hostDev)
		assert.Error(t, err, "the pair is deleted")
		assert.Equal(t, "down", status().String())
		require.NoError(t, r.Stop(ctx), "a second stop changes nothing")
		r.netErr = errors.New("misfit")
		assert.ErrorIs(t, r.ConfigError(ctx), r.netErr, "nothing held: the error is the ConfigError")
		r.netErr = nil

		require.NoError(t, r.Start(ctx))
		links, _, _, _ = guestState(t, c)
		require.Len(t, links, 1)
		assert.Equal(t, mac, links[0].Attrs().HardwareAddr.String(), "the same hardware address at the next start")

		// An explicit hardware address wins, on an interface of its own.
		r2 := newL2Resource(t, "ip#2", obj)
		r2.MacAddr, r2.NSDev = "ce:32:cc:ca:41:33", "seg1"
		require.NoError(t, r2.Configure())
		require.NoError(t, r2.Start(ctx))
		links, _, _, _ = guestState(t, c)
		require.Len(t, links, 2)
		for _, link := range links {
			if link.Attrs().Name == "seg1" {
				assert.Equal(t, "ce:32:cc:ca:41:33", link.Attrs().HardwareAddr.String())
			}
		}
		require.NoError(t, r2.Stop(ctx))
		assert.Equal(t, "up", status().String(), "the other resource of the namespace is left")

		// The network not set up on the node.
		require.NoError(t, r.Stop(ctx))
		require.NoError(t, netlink.LinkDel(br))
		assert.Equal(t, "down", status().String())
		assert.Contains(t, statusErrors(r), "the bridge obr_seg of network seg does not exist on this node")
		assert.ErrorContains(t, r.Start(ctx), "the bridge obr_seg of network seg does not exist on this node")
	})
}

// With a name and a netmask, the interface holds the address. The namespace
// gets no default route and no policy routing unless a gateway is set, then
// the default route through it. An address another namespace of the segment
// holds is refused, and what the start made is rolled back.
func TestL2StaticInNamespaces(t *testing.T) {
	if testing.Short() {
		t.Skip("announces its addresses, a second apart")
	}
	inNodeNetns(t, func(t *testing.T) {
		addBridge(t, "obr_seg")
		c1, c2, c3 := startContainer(t), startContainer(t), startContainer(t)
		obj := containerObject{containers: map[string]fakeContainer{"container#0": c1, "container#1": c2, "container#2": c3}}
		ctx := actionrollback.NewContext(context.Background())

		r1 := newL2Resource(t, "ip#1", obj)
		r1.Name, r1.Netmask = "10.99.0.5", "24"
		require.NoError(t, r1.Configure())
		require.NoError(t, r1.Start(ctx))
		links, addrs, defaults, rules := guestState(t, c1)
		require.Len(t, links, 1)
		assert.Equal(t, stableMAC(testObjectID.String(), "ip#1", "").String(), links[0].Attrs().HardwareAddr.String())
		assert.Equal(t, []string{"10.99.0.5/24"}, addrs)
		assert.Empty(t, defaults, "no default route with no gateway")
		assert.Zero(t, rules, "no policy routing with no gateway")
		r1.StatusLog().Reset()
		assert.Equal(t, "up", r1.Status(ctx).String(), statusErrors(r1))
		assert.Empty(t, statusErrors(r1))

		// The end of the node taken off the bridge: down, and plugged
		// back by a start, as for no address.
		peer1, err := netlink.LinkByName(formatHostDevName("eth0", c1.pid))
		require.NoError(t, err)
		require.NoError(t, netlink.LinkSetNoMaster(peer1))
		r1.StatusLog().Reset()
		assert.Equal(t, "down", r1.Status(ctx).String())
		assert.Contains(t, statusErrors(r1), "is not a port of obr_seg, the bridge of network seg")
		// Plugged back, it is waited for to forward, as a port made by the
		// start, before the announces.
		var waited []string
		prevWait := waitBridgePortForwarding
		waitBridgePortForwarding = func(_ *T, dev string) { waited = append(waited, dev) }
		require.NoError(t, r1.Start(ctx), "plugged back")
		waitBridgePortForwarding = prevWait
		assert.Equal(t, []string{peer1.Attrs().Name}, waited, "the port plugged back waited for")
		r1.StatusLog().Reset()
		assert.Equal(t, "up", r1.Status(ctx).String(), statusErrors(r1))

		r2 := newL2Resource(t, "ip#2", obj)
		r2.NetNS = "container#1"
		r2.Name, r2.Netmask, r2.Gateway = "10.99.0.6", "24", "10.99.0.1"
		require.NoError(t, r2.Configure())
		require.NoError(t, r2.Start(ctx))
		_, addrs, defaults, rules = guestState(t, c2)
		assert.Equal(t, []string{"10.99.0.6/24"}, addrs)
		assert.Equal(t, []string{"10.99.0.1"}, defaults, "the default route through the gateway")
		assert.Equal(t, 1, rules, "the address answers by its gateway")
		r2.StatusLog().Reset()
		assert.Equal(t, "up", r2.Status(ctx).String(), statusErrors(r2))

		// The frames pass: the second namespace resolves the first.
		path, _ := c2.NetNSPath(ctx)
		guest, err := ns.GetNS(path)
		require.NoError(t, err)
		defer guest.Close()
		require.NoError(t, guest.Do(func(ns.NetNS) error {
			hw, err := arpProbe(net.ParseIP("10.99.0.5"), "eth0")
			assert.NoError(t, err)
			assert.Equal(t, r1.macAddr(), hw.String())
			return nil
		}))

		dup := newL2Resource(t, "ip#3", obj)
		dup.NetNS = "container#2"
		dup.Name, dup.Netmask = "10.99.0.5", "24"
		require.NoError(t, dup.Configure())
		dupCtx := actionrollback.NewContext(context.Background())
		assert.ErrorContains(t, dup.Start(dupCtx), "10.99.0.5 is already held on the segment of network seg, by "+r1.macAddr())
		require.NoError(t, actionrollback.FromContext(dupCtx).Rollback(dupCtx))
		links, addrs, _, _ = guestState(t, c3)
		assert.Empty(t, links, "rolled back")
		assert.Empty(t, addrs)

		require.NoError(t, r2.Stop(ctx))
		links, _, defaults, rules = guestState(t, c2)
		assert.Empty(t, links)
		assert.Empty(t, defaults)
		assert.Zero(t, rules)
		require.NoError(t, r1.Stop(ctx))
		links, _, _, _ = guestState(t, c1)
		assert.Empty(t, links)
		all, err := netlink.LinkList()
		require.NoError(t, err)
		for _, link := range all {
			assert.NotEqual(t, "veth", link.Type(), "%s is left on the node", link.Attrs().Name)
		}
	})
}

// A hardware address changed while the interface is in the namespace, from
// the derived one to one of the macaddr keyword: the interface left under the
// former is said, a start replaces it rather than add a second one, and a
// stop deletes it.
func TestL2OnlyFormerMACInNamespaces(t *testing.T) {
	inNodeNetns(t, func(t *testing.T) {
		addBridge(t, "obr_seg")
		c := startContainer(t)
		obj := containerObject{containers: map[string]fakeContainer{"container#0": c}}
		r := newL2Resource(t, "ip#1", obj)
		require.NoError(t, r.Configure())
		derived := stableMAC(testObjectID.String(), "ip#1", "").String()
		ctx := actionrollback.NewContext(context.Background())
		status := func() status.T {
			r.StatusLog().Reset()
			return r.Status(ctx)
		}
		require.NoError(t, r.Start(ctx))

		r.MacAddr = "ce:32:cc:ca:41:33"
		require.NoError(t, r.Configure())
		assert.Equal(t, "down", status().String())
		assert.Contains(t, statusErrors(r), "with the former hardware address "+derived+", not ce:32:cc:ca:41:33")
		holds, err := r.holdsAddr(ctx)
		require.NoError(t, err)
		assert.True(t, holds, "the interface under the former address is the resource's")

		require.NoError(t, r.Start(ctx), "replaced")
		links, _, _, _ := guestState(t, c)
		require.Len(t, links, 1, "never two")
		assert.Equal(t, "ce:32:cc:ca:41:33", links[0].Attrs().HardwareAddr.String())
		assert.Equal(t, "up", status().String(), statusErrors(r))

		// Stopped: nothing left. A macaddr changed or unset while up is
		// refused by the configuration (StopToChange), no derivation
		// finding the former value back.
		require.NoError(t, r.Stop(ctx))
		links, _, _, _ = guestState(t, c)
		assert.Empty(t, links)
		all, err := netlink.LinkList()
		require.NoError(t, err)
		for _, link := range all {
			assert.NotEqual(t, "veth", link.Type(), "%s is left on the node", link.Attrs().Name)
		}
	})
}

// A static ipv6 address another namespace of the segment holds: the
// duplicate address detection of the kernel fails, so does the start, and
// the address is rolled back.
func TestL2StaticIPv6DuplicateInNamespaces(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the duplicate address detection of the kernel")
	}
	inNodeNetns(t, func(t *testing.T) {
		la := netlink.NewLinkAttrs()
		la.Name = "obr_seg"
		snooping := false
		require.NoError(t, netlink.LinkAdd(&netlink.Bridge{LinkAttrs: la, MulticastSnooping: &snooping}))
		br, err := netlink.LinkByName("obr_seg")
		require.NoError(t, err)
		require.NoError(t, netlink.LinkSetUp(br))
		c1, c2 := startContainer(t), startContainer(t)
		obj := containerObject{containers: map[string]fakeContainer{"container#0": c1, "container#1": c2}}
		ctx := actionrollback.NewContext(context.Background())

		r1 := newL2Resource(t, "ip#1", obj)
		r1.Name, r1.Netmask = "fd00::5", "64"
		require.NoError(t, r1.Configure())
		require.NoError(t, r1.Start(ctx), "the first holder passes its detection")

		dup := newL2Resource(t, "ip#2", obj)
		dup.NetNS = "container#1"
		dup.Name, dup.Netmask = "fd00::5", "64"
		require.NoError(t, dup.Configure())
		dupCtx := actionrollback.NewContext(context.Background())
		assert.ErrorContains(t, dup.Start(dupCtx), "fd00::5 is already held on the segment of network seg: its duplicate address detection failed")
		// Before the rollback, the address the kernel refused is a
		// warning: still held, so not changed from under by the
		// configuration, as a down would let it.
		dup.StatusLog().Reset()
		assert.Equal(t, "warn", dup.Status(ctx).String())
		assert.Contains(t, statusErrors(dup), "fd00::5 is already held on the segment of network seg: its duplicate address detection failed")
		require.NoError(t, actionrollback.FromContext(dupCtx).Rollback(dupCtx))
		links, _, _, _ := guestState(t, c2)
		assert.Empty(t, links, "rolled back")
		require.NoError(t, r1.Stop(ctx))
	})
}
