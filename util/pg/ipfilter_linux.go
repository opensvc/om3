//go:build linux

package pg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	sddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/command"
)

var (
	// ErrNoSlice is the error of a network filter asked of a group that
	// is no systemd slice.
	ErrNoSlice = errors.New("the process group is no systemd slice")

	// ErrNoSystemd is the error of a network filter asked of a node not
	// running systemd.
	ErrNoSystemd = errors.New("the node does not run systemd")

	// ErrDelegated is the error of a network filter asked of a group
	// delegated to a user.
	ErrDelegated = errors.New("the process group is delegated to a user, whose systemd instance may not filter the network")

	// ErrIPFilterTooLong is the error of a network filter of more ranges
	// than systemd is given in a message.
	ErrIPFilterTooLong = errors.New("the filter has too many ranges")
)

// maxIPFilterRanges is the most ranges a filter may have, the allowed and
// the denied ones together.
//
// A range is 32 bytes at most in the message giving it to systemd, and a
// message bus takes no message over 32 MiB by default, systemd none over
// 128 MiB: a longer one closes the connection with an error saying nothing
// of the filter. Half a million ranges make a message of 16 MiB at most,
// and no set of rules a man wrote comes near.
const maxIPFilterRanges = 1 << 19

// ipFilterTimeout is the time systemd is given to take a filter, or the
// cappings of a group of the v1 hierarchy.
const ipFilterTimeout = 2 * time.Minute

// ipFilterDropIns are the files systemd keeps the two runtime properties
// of the filter in, under the drop-in directory of the slice.
var ipFilterDropIns = []string{"50-IPAddressAllow.conf", "50-IPAddressDeny.conf"}

// CheckIPFilter says whether a network filter can be set on the group: nil,
// or the reason it can not.
//
// The filter is the IPAddressAllow and IPAddressDeny of the slice of the
// group, which the systemd of the system enforces. The systemd instance of a
// user has no right to attach the programs enforcing it, and takes the
// properties without a word: a group delegated to one is refused.
func (c Config) CheckIPFilter() error {
	switch {
	case c.unit() == "":
		return ErrNoSlice
	case c.Delegation != nil:
		return ErrDelegated
	case !hasSystemd():
		return ErrNoSystemd
	}
	return nil
}

// ipAddressPrefix is a range of a filter as systemd takes it over D-Bus,
// one item of the a(iayu) of IPAddressAllow and IPAddressDeny: the family
// of the address, its bytes, 4 or 16 of them, and the length of the prefix.
type ipAddressPrefix struct {
	Family    int32
	Address   []byte
	PrefixLen uint32
}

// ipFilterPrefixes reads the ranges of a list as systemd takes them. An
// address is the range of itself. The names systemctl reads in a list, as
// localhost or any, are not read: they are words of systemctl, systemd being
// given ranges only, and the callers give the ranges they stand for.
func ipFilterPrefixes(ranges []string) ([]ipAddressPrefix, error) {
	// The list is never nil: an empty array is what resets the property,
	// and it must be one of the type of the property.
	l := make([]ipAddressPrefix, 0, len(ranges))
	for _, s := range ranges {
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			addr, addrErr := netip.ParseAddr(s)
			if addrErr != nil || addr.Zone() != "" {
				return nil, fmt.Errorf("%q is no range of addresses: %w", s, err)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		prefix = prefix.Masked()
		family := int32(unix.AF_INET6)
		if prefix.Addr().Is4() {
			family = unix.AF_INET
		}
		l = append(l, ipAddressPrefix{
			Family:    family,
			Address:   prefix.Addr().AsSlice(),
			PrefixLen: uint32(prefix.Bits()),
		})
	}
	return l, nil
}

// ipFilterProperties are the properties setting the filter of the slice of
// the group to the allowed and denied ranges given, in the order systemd is
// to take them.
//
// Each list is emptied before it is set, in the same call: an array of
// ranges adds to what the slice holds, so setting alone would only ever
// widen the lists, and emptying in a call of its own would leave the units
// of the slice unfiltered until the next one, where systemd attaches the
// filter once, after the last property of a call.
//
// A list is one array of all its ranges, however long. systemd writes in
// the drop-in of the property the ranges of the last array it was given
// alone, the file of the one before replaced: of a list split over several
// arrays, the units would be filtered by all of them until the next
// daemon-reload, and by the last part after it, an allowed list cut short,
// or a denied one, which lets through what was denied.
func ipFilterProperties(allow, deny []string) ([]sddbus.Property, error) {
	if n := len(allow) + len(deny); n > maxIPFilterRanges {
		return nil, fmt.Errorf("%w: %d ranges, and a filter is %d at most", ErrIPFilterTooLong, n, maxIPFilterRanges)
	}
	props := make([]sddbus.Property, 0, 4)
	add := func(name string, ranges []string) error {
		l, err := ipFilterPrefixes(ranges)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		props = append(props, sddbus.Property{Name: name, Value: dbus.MakeVariant([]ipAddressPrefix{})})
		if len(l) > 0 {
			props = append(props, sddbus.Property{Name: name, Value: dbus.MakeVariant(l)})
		}
		return nil
	}
	if err := add("IPAddressAllow", allow); err != nil {
		return nil, err
	}
	if err := add("IPAddressDeny", deny); err != nil {
		return nil, err
	}
	return props, nil
}

// setUnitProperties sets runtime properties of a unit of the systemd of
// the system, with its SetUnitProperties method, and is replaced by the
// tests.
//
// systemd is reached as systemctl reaches it: on its own socket, which
// root alone may open and which needs no message bus running, then on the
// system bus.
var setUnitProperties = func(ctx context.Context, unit string, props ...sddbus.Property) error {
	conn, err := systemdConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.SetUnitPropertiesContext(ctx, unit, true, props...)
}

// systemdConn reaches the systemd of the system, see setUnitProperties.
func systemdConn(ctx context.Context) (*sddbus.Conn, error) {
	conn, err := sddbus.NewSystemdConnectionContext(ctx)
	if err != nil {
		var busErr error
		if conn, busErr = sddbus.NewSystemConnectionContext(ctx); busErr != nil {
			return nil, fmt.Errorf("systemd is not reachable: on its socket: %w, on the system bus: %w", err, busErr)
		}
	}
	return conn, nil
}

// SetIPFilter sets the network filter of the group: the units of its slice,
// as the service of an application or the scope of a container, exchange no
// packet with a denied range that is not also an allowed one.
//
// systemd applies the filter to the units of the slice already running, so
// it is how a filter is changed under a running resource too. A process
// written straight into the group, in no unit, is not filtered. No range
// lifts the filter.
//
// The properties are given to systemd over D-Bus, not as arguments of
// systemctl: the kernel takes no argument of a command longer than 128 KiB,
// which a list of some thousands of ranges is, and a list can not be split,
// see ipFilterProperties. They are runtime ones, as the cappings: the
// filter is set again from the configuration by each start.
//
// A filter systemd could not be given is an error, the filter in force left
// as it is: a list holding what is no range, or more ranges than
// maxIPFilterRanges, of which nothing is asked of systemd, and a systemd
// that is not reachable, does not answer in ipFilterTimeout or refuses.
func (c Config) SetIPFilter(allow, deny []string) error {
	if err := c.CheckIPFilter(); err != nil {
		return err
	}
	props, err := ipFilterProperties(allow, deny)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), ipFilterTimeout)
	defer cancel()
	if err := setUnitProperties(ctx, c.unit(), props...); err != nil {
		return fmt.Errorf("set the IPAddressAllow and IPAddressDeny of %s: %w", c.unit(), err)
	}
	return nil
}

// IPFilter returns the allowed and denied ranges of the network filter of
// the group, as systemd holds them.
func (c Config) IPFilter() (allow, deny []string, err error) {
	if err := c.CheckIPFilter(); err != nil {
		return nil, nil, err
	}
	args := []string{"show", c.unit(), "-p", "IPAddressAllow", "-p", "IPAddressDeny"}
	cmd := systemctl(
		command.WithName("systemctl"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)
	b, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(cmd.Stderr())))
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "IPAddressAllow":
			allow = strings.Fields(v)
		case "IPAddressDeny":
			deny = strings.Fields(v)
		}
	}
	return allow, deny, nil
}

// HasIPFilter says a network filter was set on the group, which a set of no
// range lifts but does not forget: the properties of the slice are kept by
// systemd until they are reverted, with the group.
func (c Config) HasIPFilter() bool {
	unit := c.unit()
	if unit == "" {
		return false
	}
	for _, name := range ipFilterDropIns {
		if _, err := os.Stat(filepath.Join(systemControlDirs[0], unit+".d", name)); err == nil {
			return true
		}
	}
	return false
}

// Dir returns the directory of the group in the unified hierarchy.
func (c Config) Dir() string {
	return filepath.Join(UnifiedPath(), c.Path())
}

// isUnitDir says the name of a directory of the hierarchy is the one of a
// unit running processes, a service or a scope, which systemd attaches the
// filter of its slice to.
func isUnitDir(name string) bool {
	return strings.HasSuffix(name, ".scope") || strings.HasSuffix(name, ".service")
}

// subDirs returns the names of the groups under the directory dir.
func subDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	l := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			l = append(l, e.Name())
		}
	}
	return l
}

// HasMembers says the slice of the group holds a unit: the service of an
// application still running, or the scope of a container, in the slice or
// in a slice under it.
//
// A group that is no unit does not count, whatever runs there: the
// lxc.payload group of a container started in no scope is under the slice,
// and systemd filters nothing of it.
func (c Config) HasMembers() bool {
	return hasUnits(c.Dir())
}

// hasUnits says the directory of a slice holds a unit, in a slice under it
// included.
func hasUnits(dir string) bool {
	for _, name := range subDirs(dir) {
		switch {
		case isUnitDir(name):
			return true
		case strings.HasSuffix(name, ".slice") && hasUnits(filepath.Join(dir, name)):
			return true
		}
	}
	return false
}

// HasProcs says the group holds processes in no unit of its slice, written
// straight into it or into a group under it that is no unit, as the ones a
// container manager makes by itself: systemd filters none of them, whatever
// the filter of the slice.
func (c Config) HasProcs() bool {
	return hasLooseProcs(c.Dir())
}

// hasLooseProcs says the directory of a slice holds processes in no unit
// of it.
func hasLooseProcs(dir string) bool {
	if hasOwnProcs(dir) {
		return true
	}
	for _, name := range subDirs(dir) {
		sub := filepath.Join(dir, name)
		switch {
		case isUnitDir(name):
		case strings.HasSuffix(name, ".slice"):
			if hasLooseProcs(sub) {
				return true
			}
		case hasAnyProcs(sub):
			return true
		}
	}
	return false
}

// hasAnyProcs says the directory of a group that is no unit holds
// processes, at any depth: the groups under it are the ones of what runs
// there, as the units of the system of a container, whatever their names.
func hasAnyProcs(dir string) bool {
	if hasOwnProcs(dir) {
		return true
	}
	for _, name := range subDirs(dir) {
		if hasAnyProcs(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

// hasOwnProcs says processes are written in the group of the directory.
func hasOwnProcs(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "cgroup.procs"))
	return err == nil && len(bytes.TrimSpace(b)) > 0
}

// Slice returns the systemd slice the group is, for a unit to be placed in
// it, and an empty string for a group that is no slice or is delegated to a
// user, whose units are the ones of another systemd instance.
func (c Config) Slice() string {
	if c.Delegation != nil {
		return ""
	}
	return c.unit()
}

// IsUnified says the node holds the unified cgroup hierarchy and nothing
// else, where the group of a unit is one directory.
func IsUnified() bool {
	return isUnified()
}
