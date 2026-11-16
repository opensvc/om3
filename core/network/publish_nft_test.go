//go:build linux

package network

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/plog"
)

// withPublishState points the state and the lock to a test directory, and
// captures the rulesets applied instead of handing them to nft.
func withPublishState(t *testing.T) *[]string {
	t.Helper()
	dir := t.TempDir()
	applied := &[]string{}
	saveDir, saveLock, saveApply, saveCount := publishStateDir, publishLockPath, applyPublishRuleset, publishRuleCount
	publishStateDir = func() string { return filepath.Join(dir, "run") }
	publishLockPath = func() string { return filepath.Join(dir, "lock") }
	applyPublishRuleset = func(_ *plog.Logger, ruleset string) error {
		*applied = append(*applied, ruleset)
		return nil
	}
	publishRuleCount = func(nftables.TableFamily, string) (int, error) { return 64, nil }
	t.Cleanup(func() {
		publishStateDir, publishLockPath, applyPublishRuleset, publishRuleCount = saveDir, saveLock, saveApply, saveCount
	})
	return applied
}

func owner(t *testing.T, path, rid string) PublishOwner {
	t.Helper()
	p, err := naming.ParsePath(path)
	require.NoError(t, err)
	return PublishOwner{Path: p, RID: rid}
}

func mustPublishes(t *testing.T, l ...string) []Publish {
	t.Helper()
	p, err := ParsePublishes(l)
	require.NoError(t, err)
	return p
}

func TestPublishRulesetForwardsAndMasqueradesTheHairpin(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("10.22.0.0/24")
	ruleset, err := publishRuleset([]publishEntry{{
		Key:       "prod/svc/web ip#0",
		Addr:      net.ParseIP("10.22.0.5").To4(),
		Subnet:    subnet.String(),
		Publishes: mustPublishes(t, "25489:80/tcp", "192.168.1.10:8443:443"),
	}})
	require.NoError(t, err)

	assert.Contains(t, ruleset, "table ip osvc-publish { }\ndelete table ip osvc-publish\n")
	assert.Contains(t, ruleset, "type nat hook prerouting priority -100;")
	assert.Contains(t, ruleset, "\t\tfib daddr type local tcp dport 25489 counter dnat to 10.22.0.5:80 comment \"prod/svc/web ip#0\"\n")
	assert.Contains(t, ruleset, "\t\tip daddr 192.168.1.10 tcp dport 8443 counter dnat to 10.22.0.5:443")
	assert.Contains(t, ruleset, "ip daddr != 127.0.0.0/8 fib daddr type local tcp dport 25489")
	assert.Contains(t, ruleset, "ct status dnat ip saddr 10.22.0.0/24 ip daddr 10.22.0.5 tcp dport 80 counter masquerade")
	assert.NotContains(t, ruleset, "table ip6 osvc-publish {\n", "no ipv6 forwarding, only the delete")
	assert.Contains(t, ruleset, "delete table ip6 osvc-publish\n")
}

func TestPublishRulesetIPv6(t *testing.T) {
	ruleset, err := publishRuleset([]publishEntry{{
		Key:       "k",
		Addr:      net.ParseIP("fd01::5"),
		Publishes: mustPublishes(t, "8080:80/udp"),
	}})
	require.NoError(t, err)
	assert.Contains(t, ruleset, "table ip6 osvc-publish {\n")
	assert.Contains(t, ruleset, "udp dport 8080 counter dnat to [fd01::5]:80")
	assert.Contains(t, ruleset, "ip6 daddr != ::1 fib daddr type local")
	assert.Contains(t, ruleset, "ip6 saddr fd01::5 ip6 daddr fd01::5 udp dport 80")
}

// TestPublishRulesetIsAcceptedByNft parses the rendered document with the
// nft binary, which reports what it would refuse without applying anything.
func TestPublishRulesetIsAcceptedByNft(t *testing.T) {
	if _, err := exec.LookPath("nft"); err != nil {
		t.Skip("nft is not installed")
	}
	ruleset, err := publishRuleset([]publishEntry{
		{Key: "prod/svc/web ip#0", Addr: net.ParseIP("10.22.0.5").To4(), Subnet: "10.22.0.0/24", Publishes: mustPublishes(t, "25489:80", "10.0.0.1:53:53/udp")},
		{Key: "prod/svc/api ip#0", Addr: net.ParseIP("fd01::5"), Subnet: "fd01::/64", Publishes: mustPublishes(t, "8080:80", "[fd00::1]:8443:443")},
	})
	require.NoError(t, err)
	checkNft(t, ruleset)
	empty, err := publishRuleset(nil)
	require.NoError(t, err)
	checkNft(t, empty)
}

// checkNft has nft check a ruleset, in a network namespace of a user
// namespace of its own when the user running the test is not privileged.
func checkNft(t *testing.T, ruleset string) {
	t.Helper()
	cmd := exec.Command("nft", "--check", "-f", "-")
	cmd.Stdin = strings.NewReader(ruleset)
	b, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(b), "Operation not permitted") {
		if _, lerr := exec.LookPath("unshare"); lerr != nil {
			t.Skip("nft --check needs privileges here")
		}
		cmd = exec.Command("unshare", "-rn", "nft", "--check", "-f", "-")
		cmd.Stdin = strings.NewReader(ruleset)
		if b, err = cmd.CombinedOutput(); err != nil && strings.Contains(string(b), "unshare failed") {
			t.Skip("nft --check needs privileges here")
		}
	}
	require.NoErrorf(t, err, "nft refused the ruleset: %s\n%s", b, ruleset)
}

func TestPublishStartStop(t *testing.T) {
	applied := withPublishState(t)
	ctx := context.Background()
	web, other := owner(t, "prod/svc/web", "ip#0"), owner(t, "prod/svc/other", "ip#0")
	addr := net.ParseIP("10.22.0.5").To4()
	_, subnet, _ := net.ParseCIDR("10.22.0.0/24")

	require.NoError(t, publishStart(ctx, nil, web, addr, subnet, mustPublishes(t, "25489:80")))
	require.Len(t, *applied, 1)
	assert.Contains(t, (*applied)[0], "dport 25489 counter dnat to 10.22.0.5:80")

	recorded, loaded, err := PublishStatus(web)
	require.NoError(t, err)
	assert.True(t, recorded)
	assert.True(t, loaded)

	err = publishStart(ctx, nil, other, net.ParseIP("10.22.0.6").To4(), subnet, mustPublishes(t, "25489:81"))
	assert.ErrorContains(t, err, "already published by prod/svc/web ip#0")
	assert.Len(t, *applied, 1, "a refused start applies nothing")

	require.NoError(t, publishStart(ctx, nil, other, net.ParseIP("10.22.0.6").To4(), subnet, mustPublishes(t, "25490:80/udp")))
	assert.Contains(t, (*applied)[1], "dnat to 10.22.0.5:80")
	assert.Contains(t, (*applied)[1], "dnat to 10.22.0.6:80")

	require.NoError(t, PublishStop(ctx, nil, web))
	assert.NotContains(t, (*applied)[2], "10.22.0.5")
	assert.Contains(t, (*applied)[2], "10.22.0.6")

	recorded, _, err = PublishStatus(web)
	require.NoError(t, err)
	assert.False(t, recorded)

	require.NoError(t, PublishStop(ctx, nil, web))
	assert.Len(t, *applied, 3, "stopping a key publishing nothing applies nothing")
}

func TestPublishStartKeepsTheStateOfAFailedApply(t *testing.T) {
	withPublishState(t)
	applyPublishRuleset = func(*plog.Logger, string) error { return assert.AnError }
	k := owner(t, "svc1", "ip#0")
	err := publishStart(context.Background(), nil, k, net.ParseIP("10.22.0.5").To4(), nil, mustPublishes(t, "25489:80"))
	assert.ErrorIs(t, err, assert.AnError)
	recorded, _, err := PublishStatus(k)
	require.NoError(t, err)
	assert.False(t, recorded)
}

func TestPublishStartRefusesAHostAddrOfAnotherFamily(t *testing.T) {
	withPublishState(t)
	err := publishStart(context.Background(), nil, owner(t, "svc1", "ip#0"), net.ParseIP("10.22.0.5").To4(), nil, mustPublishes(t, "[fd00::1]:8080:80"))
	assert.ErrorContains(t, err, "not the address family")
}

func TestCheckForwarding(t *testing.T) {
	dir := t.TempDir()
	save := forwardingFiles
	t.Cleanup(func() { forwardingFiles = save })
	v4 := filepath.Join(dir, "ip_forward")
	forwardingFiles = map[nftables.TableFamily]string{nftables.TableFamilyIPv4: v4, nftables.TableFamilyIPv6: filepath.Join(dir, "absent")}

	require.NoError(t, os.WriteFile(v4, []byte("0\n"), 0600))
	err := CheckForwarding(net.ParseIP("10.0.0.1"))
	var off *ErrForwardingOff
	require.ErrorAs(t, err, &off)
	assert.Equal(t, ErrForwardingOff{Sysctl: "net.ipv4.ip_forward", Value: "0"}, *off)
	assert.EqualError(t, err, "net.ipv4.ip_forward is 0")

	require.NoError(t, os.WriteFile(v4, []byte("1\n"), 0600))
	assert.NoError(t, CheckForwarding(net.ParseIP("10.0.0.1")))

	assert.Error(t, CheckForwarding(net.ParseIP("fd00::1")))
}

// TestPublishStateIsADirectoryPerObject pins the layout of the state: a
// directory per object, a file per resource, and nothing left once the
// object publishes nothing more.
func TestPublishStateIsADirectoryPerObject(t *testing.T) {
	withPublishState(t)
	ctx := context.Background()
	ip0, ip1 := owner(t, "test/svc/publish", "ip#0"), owner(t, "test/svc/publish", "ip#1")
	rootSvc := owner(t, "web", "ip#0")
	addr := net.ParseIP("10.22.0.5").To4()

	require.NoError(t, publishStart(ctx, nil, ip0, addr, nil, mustPublishes(t, "25489:80")))
	require.NoError(t, publishStart(ctx, nil, ip1, net.ParseIP("10.22.0.6").To4(), nil, mustPublishes(t, "25490:80")))
	require.NoError(t, publishStart(ctx, nil, rootSvc, net.ParseIP("10.22.0.7").To4(), nil, mustPublishes(t, "25491:80")))

	dir := publishStateDir()
	assert.FileExists(t, filepath.Join(dir, "test", "svc", "publish", "ip#0.json"))
	assert.FileExists(t, filepath.Join(dir, "test", "svc", "publish", "ip#1.json"))
	assert.FileExists(t, filepath.Join(dir, "root", "svc", "web", "ip#0.json"))

	require.NoError(t, PublishStop(ctx, nil, ip0))
	assert.NoFileExists(t, filepath.Join(dir, "test", "svc", "publish", "ip#0.json"))
	assert.DirExists(t, filepath.Join(dir, "test", "svc", "publish"), "ip#1 still publishes")

	require.NoError(t, PublishStop(ctx, nil, ip1))
	assert.NoDirExists(t, filepath.Join(dir, "test"), "the empty directories of the object are removed")
	assert.DirExists(t, filepath.Join(dir, "root", "svc", "web"))
	assert.DirExists(t, dir)
}

// publishStart is PublishStart for the tests checking its error only.
func publishStart(ctx context.Context, log *plog.Logger, owner PublishOwner, addr net.IP, subnet *net.IPNet, publishes []Publish) error {
	_, err := PublishStart(ctx, log, owner, addr, subnet, publishes)
	return err
}

// withHostPorts has the draws of a free port of the node return the ports
// given, in order.
func withHostPorts(t *testing.T, ports ...int) *int {
	t.Helper()
	draws := new(int)
	save := allocHostPort
	allocHostPort = func(string, net.IP) (int, error) {
		if *draws >= len(ports) {
			return 0, assert.AnError
		}
		*draws++
		return ports[*draws-1], nil
	}
	t.Cleanup(func() { allocHostPort = save })
	return draws
}

func TestPublishStartDrawsAHostPort(t *testing.T) {
	applied := withPublishState(t)
	withHostPorts(t, 30123)
	web := owner(t, "web", "ip#0")

	resolved, err := PublishStart(context.Background(), nil, web, net.ParseIP("10.22.0.5").To4(), nil, mustPublishes(t, "80/tcp", "25489:81"))
	require.NoError(t, err)
	require.Len(t, resolved, 2)
	assert.Equal(t, Publish{HostPort: 30123, Port: 80, Proto: "tcp", Auto: true}, resolved[0])
	assert.Equal(t, "25489:81/tcp", resolved[1].String())
	assert.Contains(t, (*applied)[0], "tcp dport 30123 counter dnat to 10.22.0.5:80")

	got, err := PublishResolved(web)
	require.NoError(t, err)
	assert.Equal(t, resolved, got)
}

func TestPublishStartKeepsTheHostPortOfAResourceUp(t *testing.T) {
	withPublishState(t)
	draws := withHostPorts(t, 30123, 30124)
	ctx := context.Background()
	web := owner(t, "web", "ip#0")
	addr := net.ParseIP("10.22.0.5").To4()

	_, err := PublishStart(ctx, nil, web, addr, nil, mustPublishes(t, "80"))
	require.NoError(t, err)
	resolved, err := PublishStart(ctx, nil, web, addr, nil, mustPublishes(t, "80"))
	require.NoError(t, err)
	assert.Equal(t, 30123, resolved[0].HostPort, "a start of the resource up keeps the port")
	assert.Equal(t, 1, *draws)

	require.NoError(t, PublishStop(ctx, nil, web))
	resolved, err = PublishStart(ctx, nil, web, addr, nil, mustPublishes(t, "80"))
	require.NoError(t, err)
	assert.Equal(t, 30124, resolved[0].HostPort, "a stop releases the port, the next start draws one")
}

func TestPublishStartDrawsAgainAPortAnotherResourcePublishes(t *testing.T) {
	withPublishState(t)
	draws := withHostPorts(t, 25489, 30123)
	ctx := context.Background()
	require.NoError(t, publishStart(ctx, nil, owner(t, "other", "ip#0"), net.ParseIP("10.22.0.6").To4(), nil, mustPublishes(t, "25489:80")))

	resolved, err := PublishStart(ctx, nil, owner(t, "web", "ip#0"), net.ParseIP("10.22.0.5").To4(), nil, mustPublishes(t, "80"))
	require.NoError(t, err)
	assert.Equal(t, 30123, resolved[0].HostPort)
	assert.Equal(t, 2, *draws)
}

func TestPublishStartFailsWhenNoPortCanBeDrawn(t *testing.T) {
	withPublishState(t)
	withHostPorts(t)
	_, err := PublishStart(context.Background(), nil, owner(t, "web", "ip#0"), net.ParseIP("10.22.0.5").To4(), nil, mustPublishes(t, "80"))
	assert.ErrorIs(t, err, assert.AnError)
	recorded, _, err := PublishStatus(owner(t, "web", "ip#0"))
	require.NoError(t, err)
	assert.False(t, recorded)
}

// TestAllocHostPort draws real ports from the kernel: no privilege is needed
// to bind port 0.
func TestAllocHostPort(t *testing.T) {
	for _, proto := range []string{"tcp", "udp"} {
		port, err := allocHostPort(proto, net.ParseIP("127.0.0.1"))
		require.NoError(t, err, proto)
		assert.Greater(t, port, 0, proto)
	}
}

// TestPublishStatusCountsTheRulesOfTheOwner pins that the table being loaded
// is not enough: the owner has a rule per publish, the ones of the other
// owners keeping the table loaded included.
func TestPublishStatusCountsTheRulesOfTheOwner(t *testing.T) {
	withPublishState(t)
	web := owner(t, "web", "ip#0")
	require.NoError(t, publishStart(context.Background(), nil, web, net.ParseIP("10.22.0.5").To4(), nil, mustPublishes(t, "25489:80", "25490:81")))

	var asked string
	for n, want := range map[int]bool{0: false, 1: false, 2: true} {
		publishRuleCount = func(_ nftables.TableFamily, key string) (int, error) {
			asked = key
			return n, nil
		}
		recorded, loaded, err := PublishStatus(web)
		require.NoError(t, err)
		assert.True(t, recorded)
		assert.Equal(t, want, loaded, "%d rules of 2", n)
	}
	assert.Equal(t, "web ip#0", asked)
}
