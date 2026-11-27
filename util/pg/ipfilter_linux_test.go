//go:build linux

package pg

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

const ipFilterGroup = "/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-app.1.slice"

// fakeSystemctl records the systemctl calls, and answers a show with out.
func fakeSystemctl(t *testing.T, out string) *[]string {
	t.Helper()
	calls := make([]string, 0)
	savedSystemctl, savedHasSystemd := systemctl, hasSystemd
	t.Cleanup(func() { systemctl, hasSystemd = savedSystemctl, savedHasSystemd })
	hasSystemd = func() bool { return true }
	systemctl = func(opts ...funcopt.O) *command.T {
		cmd := command.New(opts...)
		calls = append(calls, strings.Join(cmd.Cmd().Args[1:], "|"))
		return command.New(command.WithName("printf"), command.WithVarArgs("%s", out), command.WithBufferedStdout())
	}
	return &calls
}

// busCall is a SetUnitProperties call the fake of systemd was given.
type busCall struct {
	unit  string
	props []sddbus.Property
}

// names are the names of the properties of the call, in their order.
func (c busCall) names() []string {
	l := make([]string, len(c.props))
	for i, p := range c.props {
		l[i] = p.Name
	}
	return l
}

// ranges are the ranges of the property i of the call.
func (c busCall) ranges(t *testing.T, i int) []ipAddressPrefix {
	t.Helper()
	require.Less(t, i, len(c.props))
	assert.Equal(t, "a(iayu)", c.props[i].Value.Signature().String(), "the type systemd reads the property with")
	l, ok := c.props[i].Value.Value().([]ipAddressPrefix)
	require.True(t, ok)
	require.NotNil(t, l, "a nil list has no type on the bus")
	return l
}

// fakeBus records the SetUnitProperties calls in place of systemd, which
// answers them with err.
func fakeBus(t *testing.T, err error) *[]busCall {
	t.Helper()
	calls := make([]busCall, 0)
	savedSet, savedHasSystemd := setUnitProperties, hasSystemd
	t.Cleanup(func() { setUnitProperties, hasSystemd = savedSet, savedHasSystemd })
	hasSystemd = func() bool { return true }
	setUnitProperties = func(ctx context.Context, unit string, props ...sddbus.Property) error {
		_, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline, "a systemd not answering must not hang the action")
		calls = append(calls, busCall{unit: unit, props: props})
		return err
	}
	return &calls
}

// v4 and v6 are a range as systemd is given it.
func v4(prefixLen uint32, b ...byte) ipAddressPrefix {
	return ipAddressPrefix{Family: unix.AF_INET, Address: b, PrefixLen: prefixLen}
}

func v6(prefixLen uint32, s string) ipAddressPrefix {
	return ipAddressPrefix{Family: unix.AF_INET6, Address: netip.MustParseAddr(s).AsSlice(), PrefixLen: prefixLen}
}

func TestSetIPFilterEmptiesEachListThenSetsItInOneCall(t *testing.T) {
	calls := fakeBus(t, nil)
	c := Config{ID: ipFilterGroup}
	require.NoError(t, c.SetIPFilter([]string{"127.0.0.0/8", "::1/128", "10.29.0.12/32"}, []string{"0.0.0.0/0", "::/0"}))
	require.Len(t, *calls, 1, "a call emptying a list alone would leave the units unfiltered until the next")
	call := (*calls)[0]
	assert.Equal(t, "opensvc-svc.web-app.1.slice", call.unit)
	require.Equal(t, []string{"IPAddressAllow", "IPAddressAllow", "IPAddressDeny", "IPAddressDeny"}, call.names(),
		"an array adds to the list systemd holds: each list is emptied, then set")
	assert.Empty(t, call.ranges(t, 0))
	assert.Equal(t, []ipAddressPrefix{
		v4(8, 127, 0, 0, 0),
		v6(128, "::1"),
		v4(32, 10, 29, 0, 12),
	}, call.ranges(t, 1), "localhost is its two ranges, as any other")
	assert.Empty(t, call.ranges(t, 2))
	assert.Equal(t, []ipAddressPrefix{v4(0, 0, 0, 0, 0), v6(0, "::")}, call.ranges(t, 3))
}

func TestSetIPFilterOfOnlyDeniedRanges(t *testing.T) {
	calls := fakeBus(t, nil)
	require.NoError(t, Config{ID: ipFilterGroup}.SetIPFilter(nil, []string{"10.29.0.13/32"}))
	require.Len(t, *calls, 1)
	call := (*calls)[0]
	require.Equal(t, []string{"IPAddressAllow", "IPAddressDeny", "IPAddressDeny"}, call.names())
	assert.Empty(t, call.ranges(t, 0), "the allowed list a former filter left is emptied")
	assert.Empty(t, call.ranges(t, 1))
	assert.Equal(t, []ipAddressPrefix{v4(32, 10, 29, 0, 13)}, call.ranges(t, 2))
}

func TestSetIPFilterOfNoRangeLiftsTheFilter(t *testing.T) {
	calls := fakeBus(t, nil)
	require.NoError(t, Config{ID: ipFilterGroup}.SetIPFilter(nil, nil))
	require.Len(t, *calls, 1)
	call := (*calls)[0]
	require.Equal(t, []string{"IPAddressAllow", "IPAddressDeny"}, call.names())
	assert.Empty(t, call.ranges(t, 0))
	assert.Empty(t, call.ranges(t, 1))
}

func TestIPFilterPrefixes(t *testing.T) {
	l, err := ipFilterPrefixes([]string{
		"10.29.0.0/24",
		"10.29.0.12",
		"10.29.0.77/24",
		"fd01:2345:6789:2900::/64",
		"fd01:2345:6789:2900::12",
		"::ffff:10.29.0.12/128",
	})
	require.NoError(t, err)
	assert.Equal(t, []ipAddressPrefix{
		v4(24, 10, 29, 0, 0),
		v4(32, 10, 29, 0, 12),
		v4(24, 10, 29, 0, 0),
		v6(64, "fd01:2345:6789:2900::"),
		v6(128, "fd01:2345:6789:2900::12"),
		v6(128, "::ffff:10.29.0.12"),
	}, l, "an address is the range of itself, and the bits past the prefix are dropped")
	for _, p := range l {
		assert.Len(t, p.Address, map[int32]int{unix.AF_INET: 4, unix.AF_INET6: 16}[p.Family],
			"systemd refuses an address that is not of the size of its family")
	}

	l, err = ipFilterPrefixes(nil)
	require.NoError(t, err)
	assert.NotNil(t, l)
	assert.Empty(t, l)

	calls := fakeBus(t, nil)
	for _, s := range []string{"localhost", "any", "", "10.29.0.0/33", "10.29.0.0/-1", "fe80::1%eth0", "fe80::1%eth0/64", "10.29.0.12 10.29.0.13"} {
		_, err := ipFilterPrefixes([]string{"10.29.0.12/32", s})
		assert.ErrorContains(t, err, "is no range of addresses", s)
		assert.ErrorContains(t, Config{ID: ipFilterGroup}.SetIPFilter(nil, []string{s}), "IPAddressDeny: ", s)
		assert.ErrorContains(t, Config{ID: ipFilterGroup}.SetIPFilter([]string{s}, nil), "IPAddressAllow: ", s)
	}
	assert.Empty(t, *calls, "nothing is asked of systemd: the filter in force is left as it is")
}

// A list is one array, however long: of several, systemd keeps in the
// drop-in of the property the ranges of the last alone. The 10000 ranges
// here are more than twice what one argument of systemctl took.
func TestSetIPFilterOfThousandsOfRanges(t *testing.T) {
	calls := fakeBus(t, nil)
	// What an ipv6 address denied inside every address allowed after it
	// carves: 128 ranges, of 20 to 43 bytes as a text.
	n := 10000
	allow := make([]string, n)
	for i := range allow {
		allow[i] = fmt.Sprintf("fd00:%04x:%04x:%04x:%04x:%04x:%04x:%04x/128", 1+i>>16, i&0xffff, 1, 2, 3, 4, 5)
	}
	require.Greater(t, len(strings.Join(allow, " ")), 2*32*os.Getpagesize(), "MAX_ARG_STRLEN, twice")
	require.NoError(t, Config{ID: ipFilterGroup}.SetIPFilter(allow, []string{"::/0"}))
	require.Len(t, *calls, 1)
	call := (*calls)[0]
	require.Equal(t, []string{"IPAddressAllow", "IPAddressAllow", "IPAddressDeny", "IPAddressDeny"}, call.names())
	l := call.ranges(t, 1)
	require.Len(t, l, n, "one array for the list")
	assert.Equal(t, v6(128, "fd00:1:0:1:2:3:4:5"), l[0])
	assert.Equal(t, v6(128, "fd00:1:270f:1:2:3:4:5"), l[n-1])

	// The message of the call, as it is written on the bus and read back.
	msg := &dbus.Message{
		Type: dbus.TypeMethodCall,
		Headers: map[dbus.HeaderField]dbus.Variant{
			dbus.FieldPath:        dbus.MakeVariant(dbus.ObjectPath("/org/freedesktop/systemd1")),
			dbus.FieldInterface:   dbus.MakeVariant("org.freedesktop.systemd1.Manager"),
			dbus.FieldMember:      dbus.MakeVariant("SetUnitProperties"),
			dbus.FieldDestination: dbus.MakeVariant("org.freedesktop.systemd1"),
			dbus.FieldSignature:   dbus.MakeVariant(dbus.SignatureOf(call.unit, true, call.props)),
		},
		Body: []any{call.unit, true, call.props},
	}
	assert.Equal(t, "sba(sv)", dbus.SignatureOf(call.unit, true, call.props).String())
	var b bytes.Buffer
	require.NoError(t, msg.EncodeTo(&b, binary.LittleEndian))
	assert.Less(t, b.Len(), 34*n, "a range is 32 bytes of the message at most")
	read, err := dbus.DecodeMessage(&b)
	require.NoError(t, err)
	require.Len(t, read.Body, 3)
	assert.Equal(t, "opensvc-svc.web-app.1.slice", read.Body[0])
	assert.Equal(t, true, read.Body[1], "runtime properties")
	props, ok := read.Body[2].([][]any)
	require.True(t, ok)
	require.Len(t, props, 4)
	assert.Equal(t, "IPAddressAllow", props[1][0])
	v, ok := props[1][1].(dbus.Variant)
	require.True(t, ok)
	assert.Equal(t, "a(iayu)", v.Signature().String())
	ranges, ok := v.Value().([][]any)
	require.True(t, ok)
	require.Len(t, ranges, n)
	assert.Equal(t, []any{int32(unix.AF_INET6), netip.MustParseAddr("fd00:1:270f:1:2:3:4:5").AsSlice(), uint32(128)}, ranges[n-1])
}

// A filter of more ranges than a message of a bus surely takes is refused
// before systemd is asked anything.
func TestSetIPFilterRefusesMoreRangesThanAMessageTakes(t *testing.T) {
	calls := fakeBus(t, nil)
	c := Config{ID: ipFilterGroup}
	ranges := func(n int) []string {
		l := make([]string, n)
		for i := range l {
			l[i] = "fd00::1/128"
		}
		return l
	}
	require.NoError(t, c.SetIPFilter(ranges(maxIPFilterRanges-1), []string{"::/0"}))
	require.Len(t, *calls, 1)
	assert.Len(t, (*calls)[0].ranges(t, 1), maxIPFilterRanges-1)
	assert.LessOrEqual(t, 32*maxIPFilterRanges, 16<<20, "half the 32 MiB a message bus takes of a message by default")

	*calls = (*calls)[:0]
	err := c.SetIPFilter(ranges(maxIPFilterRanges), []string{"::/0"})
	assert.ErrorIs(t, err, ErrIPFilterTooLong)
	assert.ErrorContains(t, err, fmt.Sprintf("%d ranges, and a filter is %d at most", maxIPFilterRanges+1, maxIPFilterRanges))
	assert.ErrorIs(t, c.SetIPFilter(nil, ranges(maxIPFilterRanges+1)), ErrIPFilterTooLong)
	assert.Empty(t, *calls, "nothing is asked of systemd: the filter in force is left as it is")
}

// A systemd that can not be reached, or refusing, fails the set: the caller
// fails its start, or keeps the filter in force.
func TestSetIPFilterFailsWhenSystemdDoesNotTakeIt(t *testing.T) {
	refused := errors.New("systemd is not reachable: on its socket: no such file, on the system bus: no such file")
	calls := fakeBus(t, refused)
	err := Config{ID: ipFilterGroup}.SetIPFilter(nil, []string{"0.0.0.0/0"})
	assert.ErrorIs(t, err, refused)
	assert.ErrorContains(t, err, "set the IPAddressAllow and IPAddressDeny of opensvc-svc.web-app.1.slice: ")
	assert.Len(t, *calls, 1)
}

func TestSetIPFilterRefusals(t *testing.T) {
	calls := fakeBus(t, nil)
	deny := []string{"0.0.0.0/0"}

	err := Config{ID: "/opensvc/svc.web/app.1"}.SetIPFilter(nil, deny)
	assert.ErrorIs(t, err, ErrNoSlice)
	assert.ErrorIs(t, Config{}.SetIPFilter(nil, deny), ErrNoSlice)

	delegated := Config{ID: ipFilterGroup}.Delegated(Delegation{Root: "/user.slice/user-1001.slice/user@1001.service", UID: 1001, GID: 1001})
	assert.ErrorIs(t, delegated.SetIPFilter(nil, deny), ErrDelegated,
		"the systemd instance of a user takes the properties and attaches nothing")
	_, _, err = delegated.IPFilter()
	assert.ErrorIs(t, err, ErrDelegated)

	hasSystemd = func() bool { return false }
	assert.ErrorIs(t, Config{ID: ipFilterGroup}.SetIPFilter(nil, deny), ErrNoSystemd)
	assert.Empty(t, *calls, "a filter refused is not asked of systemd")
}

func TestIPFilterReadsWhatSystemdHolds(t *testing.T) {
	calls := fakeSystemctl(t, "IPAddressAllow=::1/128 127.0.0.0/8 10.29.0.12/32\nIPAddressDeny=::/0 0.0.0.0/0\n")
	allow, deny, err := Config{ID: ipFilterGroup}.IPFilter()
	require.NoError(t, err)
	assert.Equal(t, []string{"::1/128", "127.0.0.0/8", "10.29.0.12/32"}, allow)
	assert.Equal(t, []string{"::/0", "0.0.0.0/0"}, deny)
	assert.Equal(t, []string{"show|opensvc-svc.web-app.1.slice|-p|IPAddressAllow|-p|IPAddressDeny"}, *calls)

	fakeSystemctl(t, "IPAddressAllow=\nIPAddressDeny=\n")
	allow, deny, err = Config{ID: ipFilterGroup}.IPFilter()
	require.NoError(t, err)
	assert.Empty(t, allow)
	assert.Empty(t, deny)
}

func TestHasIPFilterAndMembers(t *testing.T) {
	dir := t.TempDir()
	savedDirs, savedMount := systemControlDirs, cgroupMount
	t.Cleanup(func() { systemControlDirs, cgroupMount = savedDirs, savedMount })
	systemControlDirs = []string{filepath.Join(dir, "run"), filepath.Join(dir, "etc")}
	cgroupMount = filepath.Join(dir, "cgroup")
	require.NoError(t, os.MkdirAll(cgroupMount, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cgroupMount, "cgroup.procs"), nil, 0o644))
	c := Config{ID: ipFilterGroup}

	assert.False(t, c.HasIPFilter())
	dropIns := filepath.Join(dir, "run", "opensvc-svc.web-app.1.slice.d")
	require.NoError(t, os.MkdirAll(dropIns, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "50-CPUQuota.conf"), nil, 0o644))
	assert.False(t, c.HasIPFilter(), "a capping is no filter")
	require.NoError(t, os.WriteFile(filepath.Join(dropIns, "50-IPAddressDeny.conf"), nil, 0o644))
	assert.True(t, c.HasIPFilter())

	assert.Equal(t, filepath.Join(cgroupMount, ipFilterGroup), c.Dir())
	assert.False(t, c.HasMembers(), "no group")
	require.NoError(t, os.MkdirAll(c.Dir(), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(c.Dir(), "cgroup.procs"), nil, 0o644))
	assert.False(t, c.HasMembers(), "a group holding no unit")
	assert.False(t, c.HasProcs())

	// The groups a container manager makes by itself are no unit: they
	// are no member, and their processes are in no unit.
	payload := filepath.Join(c.Dir(), "lxc.payload.c1")
	require.NoError(t, os.MkdirAll(filepath.Join(payload, "init.scope"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(c.Dir(), "lxc.monitor.c1"), 0o755))
	assert.False(t, c.HasMembers(), "a group that is no unit, and the units of the system it runs")
	assert.False(t, c.HasProcs(), "groups with no process")
	require.NoError(t, os.WriteFile(filepath.Join(payload, "init.scope", "cgroup.procs"), []byte("12\n"), 0o644))
	assert.True(t, c.HasProcs(), "processes under a group that is no unit")
	assert.False(t, c.HasMembers())
	require.NoError(t, os.RemoveAll(payload))

	unit := filepath.Join(c.Dir(), "opensvc-svc.web-app.1.service")
	require.NoError(t, os.MkdirAll(unit, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(unit, "cgroup.procs"), []byte("13\n"), 0o644))
	assert.True(t, c.HasMembers())
	assert.False(t, c.HasProcs(), "the processes of a unit are filtered")
	require.NoError(t, os.RemoveAll(unit))

	scope := filepath.Join(c.Dir(), "opensvc-svc.web-app.1-lxc.scope")
	require.NoError(t, os.MkdirAll(filepath.Join(scope, "lxc.payload.c1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scope, "lxc.payload.c1", "cgroup.procs"), []byte("14\n"), 0o644))
	assert.True(t, c.HasMembers(), "a scope")
	assert.False(t, c.HasProcs(), "the groups under a scope are the ones of the unit")
	require.NoError(t, os.RemoveAll(scope))

	require.NoError(t, os.MkdirAll(filepath.Join(c.Dir(), "opensvc-svc.web-app.1-sub.slice", "x.scope"), 0o755))
	assert.True(t, c.HasMembers(), "a unit of a slice under the slice")

	assert.Equal(t, "opensvc-svc.web-app.1.slice", c.Slice())
	assert.Empty(t, Config{ID: "/opensvc/svc.web/app.1"}.Slice(), "no slice")
	assert.Empty(t, Config{ID: ipFilterGroup, Delegation: &Delegation{}}.Slice(), "delegated to a user")
}
