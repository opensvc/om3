package netwarden

import (
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/cgroupbpf"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
)

const testGroup = "/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-app.1.slice"

// fakeSlice is a slice holding the filter it is set, as systemd does.
type fakeSlice struct {
	allow, deny []string
	sets        int
	hasFilter   bool
	members     bool
	procs       bool
	checkErr    error
	// appends makes a set add to the lists, as systemd does when it is
	// asked without emptying them.
	appends bool
}

func (t *fakeSlice) CheckIPFilter() error { return t.checkErr }
func (t *fakeSlice) HasIPFilter() bool    { return t.hasFilter }
func (t *fakeSlice) HasMembers() bool     { return t.members }
func (t *fakeSlice) HasProcs() bool       { return t.procs }
func (t *fakeSlice) Dir() string          { return "/sys/fs/cgroup" + testGroup }

func (t *fakeSlice) SetIPFilter(allow, deny []string) error {
	t.sets++
	t.hasFilter = true
	if t.appends {
		t.allow, t.deny = append(t.allow, allow...), append(t.deny, deny...)
		return nil
	}
	t.allow, t.deny = allow, deny
	return nil
}

func (t *fakeSlice) IPFilter() ([]string, []string, error) {
	return t.allow, t.deny, nil
}

type fakeQuerier map[string]int

func (t fakeQuerier) Effective(dir string, _ cgroupbpf.Direction) (int, error) {
	n, ok := t[dir]
	if !ok {
		return 0, errors.New("no such cgroup")
	}
	return n, nil
}

// setup fakes the slice, the view, the capability and the kernel, and
// returns the slice and the target of a resource.
func setup(t *testing.T) (*fakeSlice, Target) {
	t.Helper()
	s := &fakeSlice{}
	savedSliceOf, savedView, savedCap, savedQuerier, savedDir, savedLookup := sliceOf, localView, hasCapability, querier, cgroupDir, lookupObjects
	t.Cleanup(func() {
		sliceOf, localView, hasCapability, querier, cgroupDir, lookupObjects = savedSliceOf, savedView, savedCap, savedQuerier, savedDir, savedLookup
	})
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		return nil, nil, errors.New("the daemon is not asked by this test")
	}
	sliceOf = func(*pg.Config) slice { return s }
	hasCapability = func() bool { return true }
	localView = func() (netrule.View, error) {
		return netrule.Data{
			Networks: map[string]netrule.DataNetwork{
				"lan": {Ranges: []netip.Prefix{netip.MustParsePrefix("192.168.0.0/24")}},
			},
			NodeAddrs: map[string][]netip.Addr{
				"n2": {netip.MustParseAddr("10.29.0.12")},
			},
		}, nil
	}
	dir := t.TempDir()
	return s, Target{
		Path:   naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "web"},
		PG:     &pg.Config{ID: testGroup},
		VarDir: func() string { return dir },
		Log:    plog.NewDefaultLogger(),
	}
}

func TestKeywords(t *testing.T) {
	kws := Keywords("process")
	require.Len(t, kws, 1, "one keyword holds the rules, in their order")
	kw := kws[0]
	assert.Equal(t, "net_rules", kw.Option)
	assert.Equal(t, "process", kw.Group)
	assert.True(t, kw.Scopable)
	assert.NotEmpty(t, kw.Text, "the text of the keyword is embedded")
	assert.NoError(t, kw.Validate(kw.Example), "the example of the keyword is a value it takes")
	assert.NoError(t, kw.Validate("allow object ./svc/db rid ip#1 deny namespace ."))
	assert.NoError(t, kw.Validate("allow address from ./cfg/acl key k"))
	assert.Error(t, kw.Validate("allow address from ./svc/acl key k"))
	assert.Error(t, kw.Validate("node n1"), "a rule starts with its verb")
	assert.Equal(t, "Net.Rules", kw.Attr)
}

func TestEnabled(t *testing.T) {
	assert.False(t, T{}.Enabled())
	assert.False(t, T{Rules: " \n "}.Enabled())
	assert.True(t, T{Rules: "allow node n2"}.Enabled())
	assert.True(t, T{Rules: "deny any\nallow any"}.Enabled())
}

func TestApplySetsTheFilterTheRulesResolveTo(t *testing.T) {
	s, target := setup(t)
	net := T{Rules: "deny address 192.168.0.128/25\nallow node n2\nallow network lan"}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "192.168.0.0/25", "::1/128"}, s.allow)
	assert.Equal(t, []string{"0.0.0.0/0", "::/0"}, s.deny)

	infos, warnings := net.Status(target)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"netwarden: deny 192.168.0.128/25, allow 10.29.0.12/32, allow 192.168.0.0/24, implicit allow 127.0.0.0/8 ::1/128, implicit deny any"}, infos,
		"the status shows the rules in their order, the implicit ones included, where the slice holds the two lists saying the same")
}

func TestApplyOfOnlyDenyRules(t *testing.T) {
	s, target := setup(t)
	net := T{Rules: "deny node n2\nallow any"}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Empty(t, s.allow)
	assert.Equal(t, []string{"10.29.0.12/32"}, s.deny)
	infos, _ := net.Status(target)
	assert.Equal(t, []string{"netwarden: deny 10.29.0.12/32, allow any"}, infos)
}

// The use case of the keyword on a slice: a service on the lan network
// reaching its namespace, the nameservers and the world, and nothing else
// of the cluster. systemd is given the hosts allowed before a deny and the
// ranges denied, and passes the rest, which is what the last allow any
// says: every address is not carved around the denied ones.
func TestApplyOfTheServiceCutFromTheClusterAndOpenToTheWorld(t *testing.T) {
	s, target := setup(t)
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		return netrule.Objects{
			"test/svc/web": {"ip#1": {netip.MustParseAddr("192.168.0.21")}},
			"test/svc/db":  {"ip#1": {netip.MustParseAddr("192.168.0.22")}},
		}, nil, nil
	}
	// The nameserver is an address of a cluster node, as the ones of
	// cluster.dns are.
	target.DNS = func() ([]netip.Addr, error) { return []netip.Addr{netip.MustParseAddr("10.29.0.12")}, nil }
	net := T{Rules: `
		allow namespace .
		allow dns
		deny cluster
		deny network lan
		allow any
	`}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Equal(t, []string{"10.29.0.12/32", "192.168.0.21/32", "192.168.0.22/32"}, s.allow,
		"the nameserver allowed before the cluster is denied is one of the allowed list, which wins")
	assert.Equal(t, []string{"10.29.0.12/32", "192.168.0.0/24"}, s.deny)

	infos, warnings := net.Status(target)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"netwarden: allow 192.168.0.21/32 192.168.0.22/32, allow 10.29.0.12/32, deny 10.29.0.12/32, deny 192.168.0.0/24, allow any"}, infos,
		"the status shows the rules as they are resolved, in their order")

	// Without the allow dns, the deny of the cluster takes the
	// nameserver away, the implicit rule coming after it.
	_, err = T{Rules: "allow namespace .\ndeny cluster\ndeny network lan\nallow any"}.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, []string{"192.168.0.21/32", "192.168.0.22/32"}, s.allow)
	assert.Equal(t, []string{"10.29.0.12/32", "192.168.0.0/24"}, s.deny)

	// Without the last allow any, the rest is denied, and the allowed
	// list is what the allow rules name less what is denied before them.
	_, err = T{Rules: "deny address 192.168.0.22\nallow namespace .\nallow dns\ndeny cluster"}.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "192.168.0.21/32", "::1/128"}, s.allow)
	assert.Equal(t, []string{"0.0.0.0/0", "::/0"}, s.deny)
}

func TestApplyRefusesAFilterSystemdDidNotTake(t *testing.T) {
	s, target := setup(t)
	s.allow, s.appends = []string{"172.16.0.0/12"}, true
	_, err := T{Rules: "allow node n2"}.Apply(target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "netwarden: the slice opensvc-svc.web-app.1.slice holds allow 10.29.0.12/32 127.0.0.0/8 172.16.0.0/12 ::1/128, deny any after it was set allow 10.29.0.12/32 127.0.0.0/8 ::1/128, deny any")
}

func TestApplyOfRulesResolvingToNothingSetsNoFilter(t *testing.T) {
	s, target := setup(t)
	net := T{Rules: "deny network nowhere\nallow any"}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.False(t, filtered, "nothing to verify: no filter is attached")
	assert.Zero(t, s.sets)
	_, warnings := net.Status(target)
	assert.Empty(t, warnings)

	// The filter a former start left is lifted.
	s.hasFilter, s.deny = true, []string{"10.0.0.1/32"}
	filtered, err = net.Apply(target)
	require.NoError(t, err)
	assert.False(t, filtered)
	assert.Empty(t, s.deny)
}

func TestApplyWithoutRulesLiftsTheFilterAFormerConfigurationLeft(t *testing.T) {
	s, target := setup(t)
	filtered, err := T{}.Apply(target)
	require.NoError(t, err)
	assert.False(t, filtered)
	assert.Zero(t, s.sets, "no property is set on a slice that never had a filter")

	s.hasFilter, s.deny = true, []string{"0.0.0.0/0", "::/0"}
	_, err = T{}.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, 1, s.sets)
	assert.Empty(t, s.deny)
}

func TestCheckRefusals(t *testing.T) {
	s, target := setup(t)
	net := T{Rules: "allow node n2"}
	require.NoError(t, net.Check(target))
	require.NoError(t, T{}.Check(Target{}), "a resource with no rule needs nothing")

	assert.EqualError(t, T{Rules: "allow nodes n2"}.Check(target),
		`netwarden: net_rules line 1: "allow nodes n2": unknown rule, expected one of localhost, dns, any, cluster, address, network, node, object, namespace`)
	assert.EqualError(t, T{Rules: "deny any port 22 proto tcp\nallow any"}.Check(target),
		`netwarden: net_rules line 1: "deny any port 22 proto tcp": port 22: the cgroup filter of the app, task, oci and lxc container resources knows no port, only the nftables filter of container.kvm does`)
	assert.NoError(t, T{Rules: "allow address from ./cfg/acl key k"}.Check(target))
	assert.NoError(t, T{Rules: "allow object ./svc/db\nallow namespace prod"}.Check(target))

	noPG := target
	noPG.PG = nil
	assert.EqualError(t, net.Check(noPG), "netwarden: net_rules needs a process group: the process group is no systemd slice")

	for _, reason := range []error{pg.ErrNoSlice, pg.ErrNoSystemd, pg.ErrDelegated} {
		s.checkErr = reason
		err := net.Check(target)
		assert.ErrorIs(t, err, reason)
		assert.EqualError(t, err, "netwarden: net_rules can not be applied: "+reason.Error())
		_, err = net.Apply(target)
		assert.ErrorIs(t, err, reason, "a filter refused is not applied")
	}
	s.checkErr = nil
	assert.Zero(t, s.sets)

	hasCapability = func() bool { return false }
	assert.EqualError(t, net.Check(target),
		"netwarden: net_rules can not be applied: the node lacks the node.x.net_filter capability, a unified cgroup hierarchy whose kernel attaches network filters, as of the last scan")
}

func TestVerify(t *testing.T) {
	_, target := setup(t)
	base := "/sys/fs/cgroup" + testGroup
	dirs := map[int]string{
		1: base + "/docker-abc.scope",
		2: base + "/docker-def.scope",
		3: "/sys/fs/cgroup/opensvc-svc.web-app.1.slice/abc",
		4: base,
	}
	cgroupDir = func(pid int) (string, error) {
		dir, ok := dirs[pid]
		if !ok {
			return "", errors.New("no such process")
		}
		return dir, nil
	}
	querier = fakeQuerier{
		base:                       0,
		base + "/docker-abc.scope": 1,
		base + "/docker-def.scope": 0,
	}
	assert.NoError(t, Verify(target, 1))

	err := Verify(target, 2)
	assert.ErrorIs(t, err, cgroupbpf.ErrNotAttached)

	err = Verify(target, 3)
	assert.ErrorIs(t, err, cgroupbpf.ErrNotAttached, "a container the cgroupfs driver placed out of the slice")
	assert.EqualError(t, err, "netwarden: the process 3 runs in the cgroup /sys/fs/cgroup/opensvc-svc.web-app.1.slice/abc, in no unit of the slice opensvc-svc.web-app.1.slice the filter is set on: no network filter is attached")

	assert.ErrorIs(t, Verify(target, 4), cgroupbpf.ErrNotAttached, "a process written in the slice itself, in no unit")
	assert.Error(t, Verify(target, 5))
}

func TestClear(t *testing.T) {
	s, target := setup(t)
	net := T{Rules: "allow node n2"}
	_, err := net.Apply(target)
	require.NoError(t, err)
	record := filepath.Join(target.VarDir(), appliedFile)
	require.FileExists(t, record)

	s.members = true
	require.NoError(t, net.Clear(target))
	assert.NotEmpty(t, s.deny, "kept while the slice holds a unit, whose processes it filters")
	require.FileExists(t, record)

	s.members = false
	require.NoError(t, net.Clear(target))
	assert.Empty(t, s.allow)
	assert.Empty(t, s.deny)
	assert.NoFileExists(t, record)

	sets := s.sets
	s.hasFilter = false
	require.NoError(t, net.Clear(target))
	assert.Equal(t, sets, s.sets, "nothing to lift")
	require.NoError(t, net.Clear(Target{}), "a resource with no process group")
}

// The nameservers the dns rule names are the ones the resource was told at
// its start: a running guest or container keeps the resolver it was given.
// Its networks naming others since, the status says the filter still allows
// the ones of the start, and how a new one is allowed with no restart. A
// resource resolving through the nameservers of the node is told none, and
// warned of nothing.
func TestStatusWarnsOfNameserversChangedSinceTheStart(t *testing.T) {
	a, b := netip.MustParseAddr("192.168.100.53"), netip.MustParseAddr("192.168.100.54")
	net := T{Rules: "allow node n2"}
	start := func(t *testing.T, dns func() ([]netip.Addr, error)) Target {
		t.Helper()
		_, target := setup(t)
		target.DNS = dns
		_, err := net.Apply(target)
		require.NoError(t, err)
		return target
	}

	t.Run("changed", func(t *testing.T) {
		target := start(t, func() ([]netip.Addr, error) { return []netip.Addr{a}, nil })
		target.DNS = func() ([]netip.Addr, error) { return []netip.Addr{b}, nil }
		infos, warnings := net.Status(target)
		require.Len(t, warnings, 1)
		assert.Equal(t, `netwarden: the nameservers of the resource changed since it started, 192.168.100.54 where it was told 192.168.100.53: the filter allows the ones it was told until it restarts, and a rule "allow address <nameserver>" allows a new one meanwhile, the daemon applying it with no restart`, warnings[0])
		assert.Len(t, infos, 1, "the filter in force is still told")
		assert.Contains(t, infos[0], "192.168.100.53/32", "the nameserver of the start is the one allowed")
	})

	t.Run("one more, and none any more", func(t *testing.T) {
		target := start(t, func() ([]netip.Addr, error) { return []netip.Addr{a}, nil })
		target.DNS = func() ([]netip.Addr, error) { return []netip.Addr{a, b}, nil }
		_, warnings := net.Status(target)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "192.168.100.53 192.168.100.54 where it was told 192.168.100.53")
		target.DNS = func() ([]netip.Addr, error) { return nil, nil }
		_, warnings = net.Status(target)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "none where it was told 192.168.100.53")
	})

	t.Run("not told", func(t *testing.T) {
		target := start(t, func() ([]netip.Addr, error) { return []netip.Addr{a}, nil })
		target.DNS = func() ([]netip.Addr, error) { return nil, errors.New("network lan: not a network name") }
		_, warnings := net.Status(target)
		assert.Empty(t, warnings, "nameservers that could not be told are no change of them")
	})

	t.Run("the same", func(t *testing.T) {
		target := start(t, func() ([]netip.Addr, error) { return []netip.Addr{a, b}, nil })
		target.DNS = func() ([]netip.Addr, error) { return []netip.Addr{b, a, b}, nil }
		_, warnings := net.Status(target)
		assert.Empty(t, warnings, "the same nameservers, in another order")
	})

	t.Run("none of its own", func(t *testing.T) {
		target := start(t, nil)
		_, warnings := net.Status(target)
		assert.Empty(t, warnings, "the nameservers of the node, which the record does not hold")

		// A record of a start telling none, read by a driver telling
		// some: nothing was recorded to compare with.
		target.DNS = func() ([]netip.Addr, error) { return []netip.Addr{b}, nil }
		_, warnings = net.Status(target)
		assert.Empty(t, warnings)
	})
}

func TestStatusWarnsOfAFilterThatIsNotTheOneApplied(t *testing.T) {
	s, target := setup(t)
	net := T{Rules: "allow node n2"}

	infos, warnings := T{}.Status(target)
	assert.Empty(t, infos)
	assert.Empty(t, warnings, "no rule, nothing to say")

	// Rules set after the start, what runs being in a unit of the slice:
	// the daemon sets the filter under it.
	s.members = true
	_, warnings = net.Status(target)
	assert.Equal(t, []string{"netwarden: net_rules was set after the resource started, and the daemon has not applied it yet: it runs unfiltered until then"}, warnings)

	// The command of an app started with no rule is in no unit: systemd
	// filters nothing of it, whatever the daemon sets.
	s.members, s.procs = false, true
	_, warnings = net.Status(target)
	assert.Equal(t, []string{"netwarden: the rules apply at the next start: what the resource runs is in no unit of the slice"}, warnings)

	// Rules the daemon refuses.
	_, warnings = T{Rules: "allow nowhere"}.Status(target)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "netwarden: the rules are refused, and the resource runs unfiltered: net_rules line 1: ")
	s.members, s.procs = false, false

	_, err := net.Apply(target)
	require.NoError(t, err)
	_, warnings = net.Status(target)
	assert.Empty(t, warnings)

	// Lifted behind om's back, as by a revert of the slice.
	s.allow, s.deny = nil, nil
	infos, warnings = net.Status(target)
	assert.Empty(t, infos)
	assert.Equal(t, []string{"netwarden: the slice opensvc-svc.web-app.1.slice holds no filter, not the allow 10.29.0.12/32 127.0.0.0/8 ::1/128, deny any applied"}, warnings)

	// The rules changed, which the daemon applies: until it has.
	_, err = net.Apply(target)
	require.NoError(t, err)
	changed := T{Rules: "allow node n2\nallow network lan"}
	infos, warnings = changed.Status(target)
	assert.Equal(t, []string{"netwarden: net_rules changed, and the daemon has not applied it yet: the filter in force is the one of the rules before"}, warnings)
	assert.Equal(t, []string{"netwarden: allow 10.29.0.12/32, implicit allow 127.0.0.0/8 ::1/128, implicit deny any"}, infos)

	// Changed to rules the daemon refuses: it keeps the filter.
	infos, warnings = T{Rules: "allow node n2 port 22 proto tcp"}.Status(target)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "netwarden: the rules changed since the filter was set are refused, and the filter is kept: net_rules line 1: ")
	assert.Equal(t, []string{"netwarden: allow 10.29.0.12/32, implicit allow 127.0.0.0/8 ::1/128, implicit deny any"}, infos)

	// What a start resolves is what the record of the rules, resolved
	// by the daemon for a resource given rules while it runs, holds.
	rec := changed.Record(target)
	assert.Equal(t, netrule.Texts{"allow node n2", "allow network lan"}, rec.NetRules)
	assert.Equal(t, target.PG.ID, rec.PG)
	assert.Equal(t, target.Path.Namespace, rec.Namespace)
	assert.True(t, rec.Filter.IsZero())

	require.NoError(t, os.WriteFile(filepath.Join(target.VarDir(), appliedFile), []byte("{"), 0o644))
	_, warnings = net.Status(target)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "netwarden: netwarden.json: ")
}

// The status compares the rules of the configuration with the ones the
// filter was resolved from, not the text of the keyword: the same rules
// laid out another way wait for nothing of the daemon, and a rule changed
// or moved does. The record holds a rule per entry, and the one an om
// knowing net_allow and net_deny wrote is read as the same list.
func TestStatusDoesNotWarnOfALayoutOnlyChangeOfTheRules(t *testing.T) {
	_, target := setup(t)
	net := T{Rules: "deny address 192.168.0.7\nallow node n2\nallow network lan"}
	_, err := net.Apply(target)
	require.NoError(t, err)
	file := filepath.Join(target.VarDir(), appliedFile)
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	const recorded = `"rules":["deny address 192.168.0.7","allow node n2","allow network lan"]`
	assert.Contains(t, string(b), recorded)

	changed := []string{"netwarden: net_rules changed, and the daemon has not applied it yet: the filter in force is the one of the rules before"}
	check := func() {
		t.Helper()
		for _, same := range []T{
			net,
			{Rules: "deny address 192.168.0.7 allow node n2 allow network lan"},
			{Rules: "\n\tdeny address   192.168.0.7\n\n   allow node n2\n   allow network lan\n"},
		} {
			infos, warnings := same.Status(target)
			assert.Empty(t, warnings, "%q", same.Rules)
			assert.Len(t, infos, 1, "%q", same.Rules)
		}
		_, warnings := T{Rules: "deny address 192.168.0.7\nallow node n2"}.Status(target)
		assert.Equal(t, changed, warnings, "a rule removed")
		_, warnings = T{Rules: "deny address 192.168.0.7\nallow network lan\nallow node n2"}.Status(target)
		assert.Equal(t, changed, warnings, "the rules in another order")
		_, warnings = T{Rules: "deny address 192.168.0.7\nallow node n2\nallow network lan\nallow nowhere"}.Status(target)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "netwarden: the rules changed since the filter was set are refused, and the filter is kept: net_rules line 4: ")
	}
	check()

	// The record of an om knowing the two keywords, as lists then as
	// their text: the deny rules first, as they won.
	old := strings.Replace(string(b), recorded, `"net_allow":["node n2","network lan"],"net_deny":["address 192.168.0.7"]`, 1)
	require.NotEqual(t, string(b), old)
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	check()
	old = strings.Replace(string(b), recorded, `"net_allow":"\n    node n2\n    network lan","net_deny":"address 192.168.0.7"`, 1)
	require.NotEqual(t, string(b), old)
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	check()
}

func TestReadNameservers(t *testing.T) {
	p := filepath.Join(t.TempDir(), "resolv.conf")
	require.NoError(t, os.WriteFile(p, []byte("# a comment\nnameserver 127.0.0.53\nnameserver fd00::53\nnameserver fe80::1%eth0\noptions edns0\nsearch example.com\nnameserver not-an-address\n"), 0o644))
	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("127.0.0.53"),
		netip.MustParseAddr("fd00::53"),
		netip.MustParseAddr("fe80::1"),
	}, readNameservers(p))
	assert.Empty(t, readNameservers(filepath.Join(t.TempDir(), "none")))
}

// The nameservers of a command run on the node are the ones its resolv.conf
// names, a stub as the one of systemd-resolved or real servers, whatever
// the file is a link to, and never the ones of cluster.dns: a command asking
// the stub talks to no node serving the cluster dns.
func TestNodeNameservers(t *testing.T) {
	saved := resolvConf
	t.Cleanup(func() { resolvConf = saved })
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub-resolv.conf")
	require.NoError(t, os.WriteFile(stub, []byte("nameserver 127.0.0.53\noptions edns0 trust-ad\nsearch example.com\n"), 0o644))
	resolvConf = filepath.Join(dir, "resolv.conf")
	require.NoError(t, os.Symlink(stub, resolvConf))
	want := []netip.Addr{netip.MustParseAddr("127.0.0.53")}
	assert.Equal(t, want, NodeNameservers())
	assert.Equal(t, want, (&nodeView{}).DNS(), "the view of the node names no nameserver of the cluster")

	resolvConf = filepath.Join(dir, "real")
	require.NoError(t, os.WriteFile(resolvConf, []byte("nameserver 10.29.0.11\nnameserver 10.29.0.12\n"), 0o644))
	want = []netip.Addr{netip.MustParseAddr("10.29.0.11"), netip.MustParseAddr("10.29.0.12")}
	assert.Equal(t, want, (&nodeView{}).DNS(), "a node resolving through real servers has them")

	resolvConf = filepath.Join(dir, "none")
	assert.Empty(t, (&nodeView{}).DNS())
}

func TestParseNameservers(t *testing.T) {
	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("10.29.0.11"),
		netip.MustParseAddr("fd00::53"),
		netip.MustParseAddr("fe80::1"),
	}, ParseNameservers([]string{"10.29.0.11", "fd00::53", "not-an-address", "10.29.0.11", "fe80::1%eth0", "::ffff:10.29.0.11"}))
	assert.Empty(t, ParseNameservers(nil))
}

// The filter opens the nameservers of the resource it is applied to: the
// ones of the view, the node's, for a target saying none, as an app or a
// task running a command on the node, and the ones the target says, as a
// container does, in their place.
func TestApplyOpensTheNameserversOfTheResource(t *testing.T) {
	s, target := setup(t)
	view, err := localView()
	require.NoError(t, err)
	data := view.(netrule.Data)
	data.Nameservers = []netip.Addr{netip.MustParseAddr("127.0.0.53")}
	localView = func() (netrule.View, error) { return data, nil }

	net := T{Rules: "allow node n2"}
	_, err = net.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "::1/128"}, s.allow, "the stub of the node is in localhost")

	asked := 0
	target.DNS = func() ([]netip.Addr, error) {
		asked++
		return []netip.Addr{netip.MustParseAddr("192.168.100.53")}, nil
	}
	_, err = net.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "192.168.100.53/32", "::1/128"}, s.allow)
	assert.Equal(t, 1, asked)

	net = T{Rules: "deny dns\nallow node n2"}
	_, err = net.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "::1/128"}, s.allow, "the dns rule names the nameservers of the resource")

	// No rule: the nameservers are not asked for.
	asked = 0
	_, err = T{}.Apply(target)
	require.NoError(t, err)
	assert.Zero(t, asked)
}

// The rules naming objects and namespaces resolve to what the daemon says
// their resources publish: it is asked the objects the rules name, as they
// resolve for the namespace of the object, and nothing else.
func TestApplyAsksTheDaemonTheObjectsTheRulesName(t *testing.T) {
	s, target := setup(t)
	var askedObjects []string
	var askedNamespaces []string
	lookupObjects = func(objects []naming.Path, namespaces []string) (netrule.Objects, netrule.Identities, error) {
		for _, p := range objects {
			askedObjects = append(askedObjects, p.String())
		}
		askedNamespaces = namespaces
		items := api.NetworkAddressItems{
			{Path: "test/svc/db", Resources: map[string][]string{"ip#1": {"10.0.1.5"}, "ip#2": {"10.0.1.6", "fd00::6"}, "fs#1": {}}},
			{Path: "prod/svc/db", Resources: map[string][]string{"ip#1": {"10.0.2.5"}}},
			{Path: "prod/svc/api", Resources: map[string][]string{"ip#1": {"10.0.2.6"}}},
		}
		return ObjectsOf(items), IdentitiesOf(items), nil
	}
	net := T{Rules: "deny object prod/svc/api\nallow object ./svc/db rid ip#1\nallow object ./svc/nope\nallow namespace prod\nallow node n2"}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Equal(t, []string{"prod/svc/api", "test/svc/db", "test/svc/nope"}, askedObjects)
	assert.Equal(t, []string{"prod"}, askedNamespaces)
	assert.Equal(t, []string{"10.0.1.5/32", "10.0.2.5/32", "10.29.0.12/32", "127.0.0.0/8", "::1/128"}, s.allow)

	// The record is what the daemon keeps the filter current from.
	rec, err := netrule.LoadRecord(filepath.Join(target.VarDir(), appliedFile))
	require.NoError(t, err)
	assert.Equal(t, "test", rec.Namespace)
	assert.Equal(t, testGroup, rec.PG)
	assert.Equal(t, netrule.Texts{"deny object prod/svc/api", "allow object ./svc/db rid ip#1", "allow object ./svc/nope", "allow namespace prod", "allow node n2"}, rec.NetRules, "a rule per entry, in their order")
	assert.Nil(t, rec.DNS, "the nameservers of the node, read again when the rules are")
	assert.Equal(t, 3, rec.Held.Count(netrule.ActionAllow), "the denied address of the namespace allowed is one its rule resolves to")
	assert.Equal(t, 1, rec.Held.Count(netrule.ActionDeny))
	assert.Len(t, rec.Held["allow namespace prod"], 2, "the addresses are held with their rule")

	// The status shows the addresses of the namespace of the object, and
	// counts the ones of the others.
	infos, warnings := net.Status(target)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"netwarden: allow 10.0.1.5/32, allow 10.29.0.12/32, implicit allow 127.0.0.0/8 ::1/128, implicit deny any, and 2 addresses of 2 rules naming another namespace"}, infos)

	s.allow = nil
	_, warnings = net.Status(target)
	require.Len(t, warnings, 1)
	assert.NotContains(t, warnings[0], "10.0.2.", "nor does a warning show them")
	assert.Contains(t, warnings[0], "does not hold the filter applied")
}

// answered returns what the daemon answers of the objects, read from the
// body of its answer as the client reads it.
func answered(t *testing.T, body string) (netrule.Objects, netrule.Identities, error) {
	t.Helper()
	var list api.NetworkAddressList
	require.NoError(t, json.Unmarshal([]byte(body), &list))
	return ObjectsOf(list.Items), IdentitiesOf(list.Items), nil
}

// A start records each address with the object publishing it, its id and
// the version of its configuration as the daemon tells them, not its path
// alone: the record of the delete of the object the path named before is
// weighed against the holder from the start on, not from the first time
// the daemon resolves the rules again.
func TestApplyRecordsTheObjectsHoldingTheAddresses(t *testing.T) {
	_, target := setup(t)
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		return answered(t, `{"kind": "NetworkAddressList", "items": [
			{"path": "test/svc/db", "object_id": "db-id", "version": "2026-11-20T10:00:00.123456789+01:00", "resources": {"ip#1": ["10.0.1.5"]}},
			{"path": "test/svc/bad", "object_id": "bad-id", "version": "2026-11-21T10:00:00Z", "resources": {"ip#1": ["10.0.1.9"]}},
			{"path": "test/svc/anon", "resources": {"ip#1": ["10.0.1.7"]}}
		]}`)
	}
	_, err := T{Rules: "deny object ./svc/bad\nallow object ./svc/db\nallow object ./svc/anon"}.Apply(target)
	require.NoError(t, err)

	rec, err := netrule.LoadRecord(filepath.Join(target.VarDir(), appliedFile))
	require.NoError(t, err)
	version := time.Date(2026, 11, 20, 9, 0, 0, 123456789, time.UTC)
	want := netrule.Memory{
		"allow object ./svc/db":   {netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db", ObjectID: "db-id", Version: version}},
		"allow object ./svc/anon": {netip.MustParseAddr("10.0.1.7"): {Path: "test/svc/anon"}},
		"deny object ./svc/bad": {
			netip.MustParseAddr("10.0.1.9"): {Path: "test/svc/bad", ObjectID: "bad-id", Version: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)},
		},
	}
	assert.True(t, want.Equal(rec.Held), "each rule holds its addresses, the holders told with their id and version, and the one told without by its path: %+v", rec.Held)

	// The record of the delete of the object the path named before is
	// none of the holder recorded, and the one of its own delete is.
	holder := rec.Held["allow object ./svc/db"][netip.MustParseAddr("10.0.1.5")]
	assert.False(t, holder.IsDeletedBy(tombstone.Record{Path: "test/svc/db", ObjectID: "db-id", Version: version.Add(-time.Hour)}))
	assert.True(t, holder.IsDeletedBy(tombstone.Record{Path: "test/svc/db", ObjectID: "db-id", Version: version}))
}

// The daemon tells the object publishing each address, which the start
// records: a path names two objects while a node still tells its instance
// of the object deleted beside the object created again under it. The
// address of the first is recorded as its own, for the record of its delete
// to lift it, and not as one of the object the path is told with. An
// address the daemon tells no object of is recorded with its path alone.
func TestApplyRecordsTheObjectPublishingEachAddress(t *testing.T) {
	_, target := setup(t)
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		return answered(t, `{"kind": "NetworkAddressList", "items": [
			{"path": "test/svc/db", "object_id": "id2", "version": "2026-11-20T11:00:00Z", "holders": [
				{"object_id": "id1", "version": "2026-11-20T10:00:00Z", "addresses": ["10.0.1.5"]},
				{"object_id": "id2", "version": "2026-11-20T11:00:00Z", "addresses": ["10.0.1.6", "::ffff:10.0.1.8"]}
			], "resources": {"ip#1": ["10.0.1.5", "10.0.1.6", "10.0.1.7", "10.0.1.8"]}},
			{"path": "test/svc/none", "object_id": "id3", "version": "2026-11-20T11:00:00Z", "holders": [], "resources": {"ip#1": ["10.0.1.9"]}}
		]}`)
	}
	_, err := T{Rules: "allow object ./svc/db\nallow object ./svc/none"}.Apply(target)
	require.NoError(t, err)

	rec, err := netrule.LoadRecord(filepath.Join(target.VarDir(), appliedFile))
	require.NoError(t, err)
	older := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	want := netrule.Memory{
		"allow object ./svc/db": {
			netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db", ObjectID: "id1", Version: older},
			netip.MustParseAddr("10.0.1.6"): {Path: "test/svc/db", ObjectID: "id2", Version: newer},
			netip.MustParseAddr("10.0.1.7"): {Path: "test/svc/db"},
			netip.MustParseAddr("10.0.1.8"): {Path: "test/svc/db", ObjectID: "id2", Version: newer},
		},
		"allow object ./svc/none": {netip.MustParseAddr("10.0.1.9"): {Path: "test/svc/none"}},
	}
	assert.True(t, want.Equal(rec.Held), "each address is held by the object the daemon told of it: %+v", rec.Held)

	// The record of the delete of the object deleted is the one of its
	// address, and none of the address of the object created again.
	deleted := tombstone.Record{Path: "test/svc/db", ObjectID: "id1", Version: older}
	held := rec.Held["allow object ./svc/db"]
	assert.True(t, held[netip.MustParseAddr("10.0.1.5")].IsDeletedBy(deleted))
	assert.False(t, held[netip.MustParseAddr("10.0.1.6")].IsDeletedBy(deleted))
}

// What the daemon answers is read with the objects of the addresses where
// it tells them, and with the object of the path alone where it does not,
// as a daemon not upgraded yet: every address is then the one of that
// object.
func TestIdentitiesOf(t *testing.T) {
	version := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	_, identities, err := answered(t, `{"kind": "NetworkAddressList", "items": [
		{"path": "test/svc/told", "object_id": "id2", "version": "2026-11-20T11:00:00Z", "holders": [
			{"object_id": "id1", "version": "2026-11-20T10:00:00Z", "addresses": ["10.0.1.5", "not an address"]}
		], "resources": {"ip#1": ["10.0.1.5", "10.0.1.6"]}},
		{"path": "test/svc/empty", "holders": [], "resources": {"ip#1": ["10.0.1.7"]}},
		{"path": "test/svc/older", "object_id": "id3", "version": "2026-11-20T10:00:00Z", "resources": {"ip#1": ["10.0.1.8"]}},
		{"path": "test/svc/anon", "resources": {"ip#1": ["10.0.1.9"]}}
	]}`)
	require.NoError(t, err)
	assert.Equal(t, netrule.Identities{
		"test/svc/told": {ObjectID: "id2", Version: version.Add(time.Hour), Publishers: map[netip.Addr]netrule.Publisher{
			netip.MustParseAddr("10.0.1.5"): {ObjectID: "id1", Version: version},
		}},
		"test/svc/empty": {Publishers: map[netip.Addr]netrule.Publisher{}},
		"test/svc/older": {ObjectID: "id3", Version: version},
	}, identities, "a path told nothing of is not there")

	p := func(s string) naming.Path {
		path, err := naming.ParsePath(s)
		require.NoError(t, err)
		return path
	}
	view := netrule.WithAddrIdentities(netrule.Data{}, identities).(netrule.AddrIdentifier)
	for _, tc := range []struct {
		path, addr, id string
		version        time.Time
		msg            string
	}{
		{"test/svc/told", "10.0.1.5", "id1", version, "the object told of the address"},
		{"test/svc/told", "10.0.1.6", "", time.Time{}, "no object told of the address, not the one of the path"},
		{"test/svc/empty", "10.0.1.7", "", time.Time{}, "no object told of any address"},
		{"test/svc/older", "10.0.1.8", "id3", version, "the object of the path, the daemon telling no other"},
		{"test/svc/anon", "10.0.1.9", "", time.Time{}, "nothing told"},
	} {
		id, got := view.AddrIdentity(p(tc.path), netip.MustParseAddr(tc.addr))
		assert.Equal(t, tc.id, id, tc.msg)
		assert.True(t, tc.version.Equal(got), "%s: got %s", tc.msg, got)
	}
}

// A daemon not upgraded yet tells the addresses of the objects, and neither
// their id nor the version of their configuration: the start sets the same
// filter, and records the addresses with the path of their object alone,
// which the daemon completes when it resolves the rules again.
func TestApplyWithADaemonTellingNoIdentity(t *testing.T) {
	s, target := setup(t)
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		return answered(t, `{"kind": "NetworkAddressList", "items": [
			{"path": "test/svc/db", "resources": {"ip#1": ["10.0.1.5"]}}
		]}`)
	}
	net := T{Rules: "allow object ./svc/db"}
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Equal(t, []string{"10.0.1.5/32", "127.0.0.0/8", "::1/128"}, s.allow)

	rec, err := netrule.LoadRecord(filepath.Join(target.VarDir(), appliedFile))
	require.NoError(t, err)
	want := netrule.Memory{"allow object ./svc/db": {netip.MustParseAddr("10.0.1.5"): {Path: "test/svc/db"}}}
	assert.True(t, want.Equal(rec.Held), "%+v", rec.Held)

	_, warnings := net.Status(target)
	assert.Empty(t, warnings)
}

// With the daemon down the resource starts: the rules naming objects
// resolve to nothing, the others as ever.
func TestApplyWithoutTheDaemon(t *testing.T) {
	s, target := setup(t)
	asked := 0
	lookupObjects = func([]naming.Path, []string) (netrule.Objects, netrule.Identities, error) {
		asked++
		return nil, nil, errors.New("connection refused")
	}
	filtered, err := T{Rules: "allow object ./svc/db\nallow node n2"}.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Equal(t, 1, asked)
	assert.Equal(t, []string{"10.29.0.12/32", "127.0.0.0/8", "::1/128"}, s.allow)

	// Rules naming no object ask nothing of the daemon.
	_, err = T{Rules: "allow node n2"}.Apply(target)
	require.NoError(t, err)
	assert.Equal(t, 1, asked)
}

// The record of a container holds its own nameservers, none included.
func TestApplyRecordsTheNameserversOfTheResource(t *testing.T) {
	_, target := setup(t)
	target.DNS = func() ([]netip.Addr, error) { return nil, nil }
	_, err := T{Rules: "allow node n2"}.Apply(target)
	require.NoError(t, err)
	rec, err := netrule.LoadRecord(filepath.Join(target.VarDir(), appliedFile))
	require.NoError(t, err)
	assert.NotNil(t, rec.DNS)
	assert.Empty(t, rec.DNS)
}

// The addresses of a key of a datastore are read at the start, from the
// datastores of the namespace of the object, and the ones of another
// namespace sharing their keys with it. The ones of a cfg are shown by the
// status, a shared one included, and the ones of a sec counted, as the logs
// of the start do.
func TestApplyReadsTheAddressesOfAKey(t *testing.T) {
	s, target := setup(t)
	localView = func() (netrule.View, error) {
		return netrule.Data{
			NodeAddrs: map[string][]netip.Addr{"n2": {netip.MustParseAddr("10.29.0.12")}},
			Keys: map[string]map[string]string{
				"test/cfg/acl":  {"allowed": "10.1.0.0/16\n192.168.7.9\nnot an address\n"},
				"test/sec/acl":  {"peers": "10.8.2.3\n10.8.2.4\n"},
				"prod/cfg/acl":  {"allowed": "10.9.0.0/16\n"},
				"infra/cfg/acl": {"allowed": "10.10.0.0/16\n"},
				"infra/sec/acl": {"peers": "10.11.2.3\n"},
			},
			Shares: map[string][]string{
				"prod/cfg/acl":  {"dev"},
				"infra/cfg/acl": {"test"},
				"infra/sec/acl": {"*"},
			},
		}, nil
	}
	net := T{Rules: "allow address from ./cfg/acl key allowed\nallow address from ./sec/acl key peers\nallow address from prod/cfg/acl key allowed\nallow address from ./cfg/acl key nope\nallow address from infra/cfg/acl key allowed\nallow address from infra/sec/acl key peers"}
	require.NoError(t, net.Check(target))
	filtered, err := net.Apply(target)
	require.NoError(t, err)
	assert.True(t, filtered)
	assert.Equal(t, []string{"10.1.0.0/16", "10.8.2.3/32", "10.8.2.4/32", "10.10.0.0/16", "10.11.2.3/32", "127.0.0.0/8", "192.168.7.9/32", "::1/128"}, s.allow,
		"the datastore of another namespace not sharing its keys with this one is not read")

	rec, err := netrule.LoadRecord(filepath.Join(target.VarDir(), appliedFile))
	require.NoError(t, err)
	assert.Equal(t, netrule.Foreign{Secret: 2, Addrs: 3}, rec.Foreign)
	assert.True(t, rec.Held.IsZero(), "the addresses of a key are no address held of an object")

	infos, warnings := net.Status(target)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"netwarden: allow 10.1.0.0/16 192.168.7.9/32, allow 10.10.0.0/16, implicit allow 127.0.0.0/8 ::1/128, implicit deny any, and 3 addresses of 2 rules reading a sec"}, infos)

	s.allow = nil
	_, warnings = net.Status(target)
	require.Len(t, warnings, 1)
	assert.NotContains(t, warnings[0], "10.8.2.", "nor does a warning show the addresses of a sec")
	assert.Contains(t, warnings[0], "does not hold the filter applied")
}

func TestReadKeyOfADatastoreTheNodeDoesNotHold(t *testing.T) {
	_, err := readKey(naming.Path{Namespace: "netwarden-test-nowhere", Kind: naming.KindCfg, Name: "acl"}, "k", "netwarden-test-nowhere")
	assert.ErrorIs(t, err, netrule.ErrNoDatastore)
}

// The node reads a key for an object of the namespace of the datastore, and
// of the namespaces its share keyword names, as the keys a resource
// installs are read: the sharing is read with the key, at every read.
func TestReadKeyOfADatastoreHonoursItsSharing(t *testing.T) {
	testhelper.Setup(t)
	acl := naming.Path{Namespace: "infra", Kind: naming.KindCfg, Name: "acl"}
	cfg, err := object.NewCfg(acl)
	require.NoError(t, err)
	require.NoError(t, cfg.AddKey("peers", []byte("10.11.2.3\n")))
	share := func(value string) {
		require.NoError(t, cfg.Config().Set(keyop.T{Key: key.Parse("share"), Op: keyop.Set, Value: value}))
		require.NoError(t, cfg.Config().Commit())
	}
	read := func(name, namespace string) (string, error) {
		view := &nodeView{}
		b, err := view.Key(acl, name, namespace)
		return string(b), err
	}

	s, err := read("peers", "infra")
	require.NoError(t, err)
	assert.Equal(t, "10.11.2.3\n", s, "its own namespace needs no share")
	_, err = read("peers", "test")
	assert.ErrorIs(t, err, netrule.ErrNotShared)
	_, err = read("nope", "test")
	assert.ErrorIs(t, err, netrule.ErrNotShared, "not shared is said before the key is looked for: nothing is told of the keys")

	share("dev test")
	s, err = read("peers", "test")
	require.NoError(t, err)
	assert.Equal(t, "10.11.2.3\n", s)
	_, err = read("nope", "test")
	assert.ErrorIs(t, err, netrule.ErrNoKey)
	_, err = read("peers", "prod")
	assert.ErrorIs(t, err, netrule.ErrNotShared)

	share("*")
	_, err = read("peers", "prod")
	assert.NoError(t, err)

	share("dev")
	_, err = read("peers", "test")
	assert.ErrorIs(t, err, netrule.ErrNotShared, "a share taken back is a key no longer read")
}
