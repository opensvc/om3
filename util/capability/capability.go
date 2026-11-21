// Package capability names the Linux capabilities and parses a capability
// bounding set as the capability_bounding_set keyword writes it, after the
// CapabilityBoundingSet of systemd: the capabilities the command keeps, all
// others dropped, or, the list prefixed with ~, the capabilities it drops.
package capability

import (
	"fmt"
	"slices"
	"strings"
)

// names are the Linux capabilities, by number, as linux/capability.h names
// them.
var names = []string{
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_DAC_READ_SEARCH",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_KILL",
	"CAP_SETGID",
	"CAP_SETUID",
	"CAP_SETPCAP",
	"CAP_LINUX_IMMUTABLE",
	"CAP_NET_BIND_SERVICE",
	"CAP_NET_BROADCAST",
	"CAP_NET_ADMIN",
	"CAP_NET_RAW",
	"CAP_IPC_LOCK",
	"CAP_IPC_OWNER",
	"CAP_SYS_MODULE",
	"CAP_SYS_RAWIO",
	"CAP_SYS_CHROOT",
	"CAP_SYS_PTRACE",
	"CAP_SYS_PACCT",
	"CAP_SYS_ADMIN",
	"CAP_SYS_BOOT",
	"CAP_SYS_NICE",
	"CAP_SYS_RESOURCE",
	"CAP_SYS_TIME",
	"CAP_SYS_TTY_CONFIG",
	"CAP_MKNOD",
	"CAP_LEASE",
	"CAP_AUDIT_WRITE",
	"CAP_AUDIT_CONTROL",
	"CAP_SETFCAP",
	"CAP_MAC_OVERRIDE",
	"CAP_MAC_ADMIN",
	"CAP_SYSLOG",
	"CAP_WAKE_ALARM",
	"CAP_BLOCK_SUSPEND",
	"CAP_AUDIT_READ",
	"CAP_PERFMON",
	"CAP_BPF",
	"CAP_CHECKPOINT_RESTORE",
}

// Name returns the name of the capability of number n, or "cap_<n>" for a
// number this package does not name, as one of a kernel newer than it.
func Name(n int) string {
	if n >= 0 && n < len(names) {
		return names[n]
	}
	return fmt.Sprintf("cap_%d", n)
}

// Number returns the number of the capability named s, in any case, and
// false when s names none.
func Number(s string) (int, bool) {
	i := slices.Index(names, strings.ToUpper(s))
	return i, i >= 0
}

// BoundingSet is a capability bounding set: the capabilities it lists, and
// whether they are the ones kept or the ones dropped.
type BoundingSet struct {
	// Drop says the listed capabilities are the ones dropped, all others
	// kept: the list was prefixed with ~.
	Drop bool

	// Caps are the numbers of the listed capabilities, sorted.
	Caps []int
}

// ParseBoundingSet parses s, a list of capability names separated by
// spaces, optionally prefixed with ~. A name it does not know is an error,
// so a typo does not leave the command with a capability its configuration
// meant to drop. An empty s is the empty set kept: every capability is
// dropped.
func ParseBoundingSet(s string) (BoundingSet, error) {
	var bs BoundingSet
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "~"); ok {
		bs.Drop = true
		s = rest
	}
	var unknown []string
	for _, name := range strings.Fields(s) {
		n, ok := Number(name)
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		if !slices.Contains(bs.Caps, n) {
			bs.Caps = append(bs.Caps, n)
		}
	}
	if len(unknown) > 0 {
		return BoundingSet{}, fmt.Errorf("unknown capabilities: %s", strings.Join(unknown, " "))
	}
	slices.Sort(bs.Caps)
	return bs, nil
}

// Keeps says whether the bounding set keeps the capability of number n.
func (t BoundingSet) Keeps(n int) bool {
	return slices.Contains(t.Caps, n) != t.Drop
}

// String returns the bounding set as ParseBoundingSet parses it back, the
// names in upper case and in number order.
func (t BoundingSet) String() string {
	l := make([]string, len(t.Caps))
	for i, n := range t.Caps {
		l[i] = Name(n)
	}
	s := strings.Join(l, " ")
	if t.Drop {
		return "~" + s
	}
	return s
}
