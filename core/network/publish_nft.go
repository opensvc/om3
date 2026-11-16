//go:build linux

package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/nftables"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/lock"
	"github.com/opensvc/om3/v3/util/plog"
)

// PublishTableName is the table the forwardings of the published ports go in.
//
// It is not the table of the networks: a network setup deletes and defines
// that one again in full, and the forwardings of the running resources would
// go with it.
const PublishTableName = "osvc-publish"

// fwPriorityDstNAT is the number the "dstnat" name stands for in the ip and
// ip6 families, written as a number for the reason the other priorities are.
const fwPriorityDstNAT = -100

// maxHostPortDraws is how many ports a start draws before giving up finding
// one no other resource of the node publishes.
const maxHostPortDraws = 64

type (
	// PublishOwner is the resource the publishes of an entry are of.
	PublishOwner struct {
		Path naming.Path
		RID  string
	}

	// publishEntry is what a resource publishing ports records on the node:
	// the address it forwards to, the network it is on, and its publishes.
	publishEntry struct {
		Key       string    `json:"key"`
		Addr      net.IP    `json:"addr"`
		Subnet    string    `json:"subnet,omitempty"`
		Publishes []Publish `json:"publishes"`
	}
)

// String is the key of the owner, the comment of its rules.
func (t PublishOwner) String() string {
	return t.Path.String() + " " + t.RID
}

// dir is the directory of the entries of the object, one per resource.
func (t PublishOwner) dir() string {
	ns := t.Path.Namespace
	if ns == "" {
		ns = naming.NsRoot
	}
	return filepath.Join(publishStateDir(), ns, t.Path.Kind.String(), t.Path.Name)
}

// file is the entry of the resource.
func (t PublishOwner) file() string {
	return filepath.Join(t.dir(), t.RID+".json")
}

var (
	// publishStateDir holds a directory per object publishing ports, with a
	// file per resource: <namespace>/<kind>/<name>/<rid>.json. It is in
	// the run directory, emptied at boot as the ruleset is, so no forwarding
	// outlives the address it goes to.
	publishStateDir = func() string { return filepath.Join(rawconfig.Paths.Run, "publish") }

	// publishLockPath serializes the changes of the state and the rendering
	// of the ruleset, between the resources starting and stopping at once.
	publishLockPath = func() string { return filepath.Join(rawconfig.Paths.Lock, "publish") }

	// applyPublishRuleset hands a ruleset to nft, replaced by the tests.
	applyPublishRuleset = func(log *plog.Logger, ruleset string) error {
		h := newNFTHandle()
		h.SetLogger(log)
		return h.apply(ruleset)
	}

	// publishTableExists says the table of a family is loaded, replaced by
	// the tests.
	publishTableExists = func(family nftables.TableFamily) (bool, error) {
		h := newNFTHandle()
		tables, err := h.Tables()
		if err != nil {
			return false, err
		}
		for _, table := range tables {
			if table.Family == family && table.Name == PublishTableName {
				return true, nil
			}
		}
		return false, nil
	}

	// allocHostPort draws a free port of the node for a publish naming none,
	// replaced by the tests. The kernel picks it: a socket bound to port 0
	// gets a port no other socket of the protocol holds.
	allocHostPort = func(proto string, addr net.IP) (int, error) {
		host := ""
		if addr != nil {
			host = addr.String()
		}
		hostport := net.JoinHostPort(host, "0")
		if proto == "udp" {
			c, err := net.ListenPacket("udp", hostport)
			if err != nil {
				return 0, err
			}
			defer c.Close()
			return c.LocalAddr().(*net.UDPAddr).Port, nil
		}
		l, err := net.Listen("tcp", hostport)
		if err != nil {
			return 0, err
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port, nil
	}

	// forwardingFiles are the sysctls saying whether the node forwards the
	// packets of a family, replaced by the tests.
	forwardingFiles = map[nftables.TableFamily]string{
		nftables.TableFamilyIPv4: "/proc/sys/net/ipv4/ip_forward",
		nftables.TableFamilyIPv6: "/proc/sys/net/ipv6/conf/all/forwarding",
	}
	forwardingSysctls = map[nftables.TableFamily]string{
		nftables.TableFamilyIPv4: "net.ipv4.ip_forward",
		nftables.TableFamilyIPv6: "net.ipv6.conf.all.forwarding",
	}
)

// ErrForwardingOff is the node not forwarding the packets of a family, so
// not the ones of the published ports either. It says the fact only: the
// caller words it for where it is shown, and the publish keyword documents
// the fix.
type ErrForwardingOff struct {
	// Sysctl is the sysctl to set to 1, as net.ipv4.ip_forward.
	Sysctl string

	// Value is the value it has.
	Value string
}

func (t *ErrForwardingOff) Error() string {
	return fmt.Sprintf("%s is %s", t.Sysctl, t.Value)
}

// CheckForwarding returns an *ErrForwardingOff when the node does not
// forward the packets of the family of ip: the published ports are forwarded
// by the node, and a node not forwarding drops them silently.
//
// om does not set the sysctl itself: it turns the node into a router, which
// is the administrator's decision.
func CheckForwarding(ip net.IP) error {
	family := ipFamily(ip)
	b, err := os.ReadFile(forwardingFiles[family])
	if err != nil {
		return err
	}
	if v := strings.TrimSpace(string(b)); v != "1" {
		return &ErrForwardingOff{Sysctl: forwardingSysctls[family], Value: v}
	}
	return nil
}

// PublishStart forwards the publishes to addr, on the network subnet, and
// records them under key, the resource they are of. It refuses a port of the
// node another resource already publishes. Starting again with other
// publishes replaces the ones of the key.
func PublishStart(ctx context.Context, log *plog.Logger, owner PublishOwner, addr net.IP, subnet *net.IPNet, publishes []Publish) ([]Publish, error) {
	if addr == nil {
		return nil, errors.New("publish: no address to forward to")
	}
	key := owner.String()
	family := ipFamily(addr)
	for _, p := range publishes {
		if p.HostAddr != nil && ipFamily(p.HostAddr) != family {
			return nil, fmt.Errorf("publish %s: not the address family of %s", p, addr)
		}
	}
	unlock, err := lock.Exclusive(ctx, publishLockPath())
	if err != nil {
		return nil, err
	}
	defer unlock()
	entries, err := loadPublishEntries()
	if err != nil {
		return nil, err
	}
	var previous *publishEntry
	others := make([]publishEntry, 0, len(entries))
	for i := range entries {
		if entries[i].Key == key {
			previous = &entries[i]
			continue
		}
		others = append(others, entries[i])
	}
	for _, o := range others {
		if ipFamily(o.Addr) != family {
			continue
		}
		for _, op := range o.Publishes {
			for _, p := range publishes {
				if p.conflicts(op) {
					return nil, fmt.Errorf("publish %s: port already published by %s", p, o.Key)
				}
			}
		}
	}
	publishes, err = resolvePublishes(publishes, previous, others, family)
	if err != nil {
		return nil, err
	}
	entry := publishEntry{Key: key, Addr: addr, Publishes: publishes}
	if subnet != nil {
		// The network, the address of the resource with its mask given
		// as well.
		entry.Subnet = (&net.IPNet{IP: subnet.IP.Mask(subnet.Mask), Mask: subnet.Mask}).String()
	}
	if err := writePublishEntry(owner, entry); err != nil {
		return nil, err
	}
	ruleset, err := publishRuleset(append(others, entry))
	if err == nil {
		err = applyPublishRuleset(log, ruleset)
	}
	if err != nil {
		// Leave the state as the ruleset still is.
		if previous != nil {
			_ = writePublishEntry(owner, *previous)
		} else {
			_ = removePublishEntry(owner)
		}
		return nil, fmt.Errorf("publish: %w", err)
	}
	return publishes, nil
}

// resolvePublishes gives the publishes naming no host port one: the port
// the previous start of the resource drew for the same declaration, so a
// start of a resource already up keeps it, else a free port of the node no
// other resource publishes.
func resolvePublishes(publishes []Publish, previous *publishEntry, others []publishEntry, family nftables.TableFamily) ([]Publish, error) {
	resolved := make([]Publish, 0, len(publishes))
	used := func(p Publish) bool {
		for _, o := range others {
			if ipFamily(o.Addr) != family {
				continue
			}
			for _, op := range o.Publishes {
				if p.conflicts(op) {
					return true
				}
			}
		}
		for _, op := range publishes {
			if p.conflicts(op) {
				return true
			}
		}
		for _, op := range resolved {
			if p.conflicts(op) {
				return true
			}
		}
		return false
	}
	for _, p := range publishes {
		if !p.Random() {
			resolved = append(resolved, p)
			continue
		}
		drawn := p
		drawn.Auto = true
		if previous != nil {
			for _, op := range previous.Publishes {
				if op.Auto && op.Declared().String() == p.String() {
					drawn.HostPort = op.HostPort
					break
				}
			}
		}
		if drawn.HostPort != 0 && !used(drawn) {
			resolved = append(resolved, drawn)
			continue
		}
		drawn.HostPort = 0
		for i := 0; i < maxHostPortDraws && drawn.HostPort == 0; i++ {
			port, err := allocHostPort(p.Proto, p.HostAddr)
			if err != nil {
				return nil, fmt.Errorf("publish %s: draw a port of the node: %w", p, err)
			}
			drawn.HostPort = port
			if used(drawn) {
				drawn.HostPort = 0
			}
		}
		if drawn.HostPort == 0 {
			return nil, fmt.Errorf("publish %s: no free port of the node drawn in %d tries", p, maxHostPortDraws)
		}
		resolved = append(resolved, drawn)
	}
	return resolved, nil
}

// PublishResolved returns the publishes of owner as its start resolved
// them, the drawn host ports given, or nil when it publishes nothing.
func PublishResolved(owner PublishOwner) ([]Publish, error) {
	entry, err := readPublishEntry(owner)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entry.Publishes, nil
}

// PublishStop removes the forwardings of owner. A resource publishing
// nothing has nothing to remove.
func PublishStop(ctx context.Context, log *plog.Logger, owner PublishOwner) error {
	key := owner.String()
	if _, err := os.Stat(owner.file()); errors.Is(err, os.ErrNotExist) {
		// The stop of every ip resource comes here, the ones never
		// publishing anything included: they take no lock.
		return nil
	}
	unlock, err := lock.Exclusive(ctx, publishLockPath())
	if err != nil {
		return err
	}
	defer unlock()
	entries, err := loadPublishEntries()
	if err != nil {
		return err
	}
	others := slices.DeleteFunc(slices.Clone(entries), func(e publishEntry) bool { return e.Key == key })
	if len(others) == len(entries) {
		return nil
	}
	ruleset, err := publishRuleset(others)
	if err != nil {
		return err
	}
	if err := applyPublishRuleset(log, ruleset); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return removePublishEntry(owner)
}

// PublishStatus says whether the forwardings of owner are recorded and their
// table loaded. An owner recorded with a table missing, as one an
// administrator flushed, is forwarded no more.
func PublishStatus(owner PublishOwner) (recorded bool, loaded bool, err error) {
	entry, err := readPublishEntry(owner)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	loaded, err = publishTableExists(ipFamily(entry.Addr))
	return true, loaded, err
}

func readPublishEntry(owner PublishOwner) (publishEntry, error) {
	var entry publishEntry
	b, err := os.ReadFile(owner.file())
	if err != nil {
		return entry, err
	}
	err = json.Unmarshal(b, &entry)
	return entry, err
}

func writePublishEntry(owner PublishOwner, entry publishEntry) error {
	if err := os.MkdirAll(owner.dir(), 0700); err != nil {
		return err
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	p := owner.file()
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// removePublishEntry removes the entry of the resource, and the directories
// of the object left empty, so an object publishing nothing more leaves
// nothing behind.
func removePublishEntry(owner PublishOwner) error {
	err := os.Remove(owner.file())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	root := filepath.Clean(publishStateDir())
	for dir := owner.dir(); dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			// Not empty: another resource of the object, or another
			// object of the namespace, publishes.
			break
		}
	}
	return nil
}

// loadPublishEntries returns the recorded entries, in the order of their
// keys, so the ruleset renders the same from the same state.
func loadPublishEntries() ([]publishEntry, error) {
	var files []string
	err := filepath.WalkDir(publishStateDir(), func(p string, d os.DirEntry, err error) error {
		switch {
		case errors.Is(err, os.ErrNotExist):
			return nil
		case err != nil:
			return err
		case !d.IsDir() && filepath.Ext(p) == ".json":
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	entries := make([]publishEntry, 0, len(files))
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var entry publishEntry
		if err := json.Unmarshal(b, &entry); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if entry.Addr == nil {
			continue
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b publishEntry) int { return strings.Compare(a.Key, b.Key) })
	return entries, nil
}

// publishRuleset renders the tables of the forwardings, as one transaction:
// a table is deleted and defined again in full, and a family left with no
// forwarding has its table deleted.
//
// For every publish:
//
//   - prerouting forwards the connections arriving to the port of the node,
//     on the address of the publish, else on any address of the node;
//   - output does the same for the connections the node itself opens to one
//     of its addresses, the loopback ones excluded: a packet sent from a
//     loopback address to the namespace would be dropped as a martian;
//   - postrouting masquerades the connections coming from the network of
//     the address, so a client on the same bridge, the namespace itself
//     included, gets its answers back through the node.
func publishRuleset(entries []publishEntry) (string, error) {
	var sb strings.Builder
	for _, family := range fwFamilies {
		name := fmtFamily(family)
		fmt.Fprintf(&sb, "table %s %s { }\n", name, PublishTableName)
		fmt.Fprintf(&sb, "delete table %s %s\n", name, PublishTableName)
		var pre, out, post strings.Builder
		for _, entry := range entries {
			if ipFamily(entry.Addr) != family {
				continue
			}
			src := entry.Subnet
			if src == "" {
				src = entry.Addr.String()
			} else if _, _, err := net.ParseCIDR(src); err != nil {
				return "", fmt.Errorf("publish %s: subnet: %w", entry.Key, err)
			}
			to := entry.Addr.String()
			loopback := "127.0.0.0/8"
			if family == nftables.TableFamilyIPv6 {
				to = "[" + to + "]"
				loopback = "::1"
			}
			comment := publishComment(entry.Key)
			for _, p := range entry.Publishes {
				dst := "fib daddr type local"
				if p.HostAddr != nil {
					dst = fmt.Sprintf("%s daddr %s", name, p.HostAddr)
				}
				dnat := fmt.Sprintf("%s dport %d counter dnat to %s:%d comment %s", p.Proto, p.HostPort, to, p.Port, comment)
				fmt.Fprintf(&pre, "\t\t%s %s\n", dst, dnat)
				fmt.Fprintf(&out, "\t\t%s daddr != %s %s %s\n", name, loopback, dst, dnat)
				fmt.Fprintf(&post, "\t\tct status dnat %s saddr %s %s daddr %s %s dport %d counter masquerade comment %s\n", name, src, name, entry.Addr, p.Proto, p.Port, comment)
			}
		}
		if pre.Len() == 0 {
			continue
		}
		fmt.Fprintf(&sb, "table %s %s {\n", name, PublishTableName)
		fmt.Fprintf(&sb, "\tchain prerouting {\n\t\ttype nat hook prerouting priority %d; policy accept;\n%s\t}\n", fwPriorityDstNAT, pre.String())
		fmt.Fprintf(&sb, "\tchain output {\n\t\ttype nat hook output priority %d; policy accept;\n%s\t}\n", fwPriorityDstNAT, out.String())
		fmt.Fprintf(&sb, "\tchain postrouting {\n\t\ttype nat hook postrouting priority %d; policy accept;\n%s\t}\n", fwPrioritySrcNAT, post.String())
		sb.WriteString("}\n")
	}
	return sb.String(), nil
}

// publishComment quotes the key of a resource for a rule comment, which
// says what a rule is for to the administrator listing the table.
func publishComment(key string) string {
	return `"` + strings.Map(func(r rune) rune {
		if r < ' ' || r > '~' || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, key) + `"`
}
