//go:build linux

package resip

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// SourceRoute is the policy routing of an address of a network
	// namespace: the address answers by the gateway of its own network,
	// whatever the default route of the namespace, which belongs to one
	// address only. A namespace holding addresses of several networks had
	// the replies of the others leave by that default route, out of an
	// interface the requests did not come in by, and the strict reverse
	// path filter of the node, or its forward chain, dropped them.
	//
	// The rule "from <address> lookup <table>" sends what the address
	// sends to a table of its own, holding the route to its network and a
	// default route by its gateway. With no gateway known, the table holds
	// no default route, and the lookup falls through to the main table, as
	// before.
	SourceRoute struct {
		// Addr is the address, with the prefix of its connected route.
		Addr *net.IPNet

		// Dev is the name of the card holding the address in the
		// namespace.
		Dev string

		// Network is the om network the address is drawn from, nil when
		// unknown.
		Network *net.IPNet

		// Gateway is the gateway of the address, nil when none is known.
		Gateway net.IP

		// OnLink says the connected route of the address is removed, as
		// the del_net_route keyword of ip.netns asks: the gateway is
		// reached on the link.
		OnLink bool
	}
)

const (
	// SourceRulePriority is the priority of the rules of the addresses,
	// before the lookup of the main table at 32766. A rule a user adds
	// with no priority lands before it, and wins. The rules of this
	// priority are om's: a rule of another priority, as one of an ip.rule
	// resource, is left alone.
	SourceRulePriority = 32000
)

// maxReservedTable is the highest of the tables the kernel names: local 255,
// main 254, default 253, unspec 0, and the ones under them a distribution
// may name in rt_tables.
const maxReservedTable = 255

var (
	ErrSourceTableInUse = errors.New("the table is used by routes of another card")
)

// TableID is the routing table of the address: the address itself, as a
// number, for an ipv4 one, and a hash of it, its high bit set, for an ipv6
// one. Unique per address, so two addresses sharing a card have a table each,
// found again from the address alone, and clear of the tables the kernel and
// the users name, all under 2^31 in practice for ipv6, and far above the
// small ones for ipv4.
func TableID(ip net.IP) int {
	if v4 := ip.To4(); v4 != nil {
		// An address of 0.0.0.0/24 names a table of the kernel, as the
		// main one: see hasTable.
		return int(binary.BigEndian.Uint32(v4))
	}
	h := fnv.New32a()
	_, _ = h.Write(ip.To16())
	return int(h.Sum32() | 0x80000000)
}

// hasTable says the address has a table of its own. An address of
// 0.0.0.0/24, whose number names a table of the kernel, as the main one, has
// none, and no policy routing: every other number is the one of another
// address, and an address of 0.0.0.0/8 is no source a host sends from.
func (t SourceRoute) hasTable() bool {
	return t.Addr != nil && TableID(t.Addr.IP) > maxReservedTable
}

// IPv6 says the address is an ipv6 one.
func (t SourceRoute) IPv6() bool {
	return t.Addr != nil && t.Addr.IP.To4() == nil
}

func (t SourceRoute) family() int {
	if t.IPv6() {
		return netlink.FAMILY_V6
	}
	return netlink.FAMILY_V4
}

// Src is the source the rule of the address matches: the address alone.
func (t SourceRoute) Src() *net.IPNet {
	if t.IPv6() {
		return &net.IPNet{IP: t.Addr.IP.To16(), Mask: net.CIDRMask(128, 128)}
	}
	return &net.IPNet{IP: t.Addr.IP.To4(), Mask: net.CIDRMask(32, 32)}
}

// connected is the route the kernel adds along with the address: the
// address masked.
func (t SourceRoute) connected() *net.IPNet {
	return &net.IPNet{IP: t.Addr.IP.Mask(t.Addr.Mask), Mask: t.Addr.Mask}
}

// Routes returns the routes of the table of the address, the card of index
// linkIndex: the connected route of the address, unless removed, the route
// to its network when the connected route covers a part of it only, and the
// default route by its gateway, when known.
func (t SourceRoute) Routes(table, linkIndex int) []netlink.Route {
	var l []netlink.Route
	connected := t.connected()
	if !t.OnLink {
		l = append(l, netlink.Route{
			LinkIndex: linkIndex,
			Dst:       connected,
			Src:       t.Addr.IP,
			Scope:     netlink.SCOPE_LINK,
			Table:     table,
		})
	}
	if t.Gateway == nil {
		return l
	}
	if NetworkRouteNeeded(t.Network, connected, t.Gateway) {
		l = append(l, netlink.Route{
			LinkIndex: linkIndex,
			Dst:       t.Network,
			Gw:        t.Gateway,
			Table:     table,
		})
	}
	if !t.OnLink && !connected.Contains(t.Gateway) {
		// A gateway out of the connected route is not reachable.
		return l
	}
	route := netlink.Route{
		LinkIndex: linkIndex,
		Dst:       anyNet(t.IPv6()),
		Gw:        t.Gateway,
		Table:     table,
	}
	if t.OnLink {
		route.Flags = int(netlink.FLAG_ONLINK)
	}
	return append(l, route)
}

// NetworkRouteNeeded says the namespace needs a route to the network through
// the gateway: the connected route of the address covers a part of the
// network only, as the range of a node of a routed_bridge network does, and
// the gateway is in that connected route. A bridge or a lan network has its
// whole segment connected.
func NetworkRouteNeeded(network, connected *net.IPNet, gateway net.IP) bool {
	if network == nil || connected == nil || connected.IP == nil || connected.Mask == nil || gateway == nil {
		return false
	}
	base := &net.IPNet{IP: connected.IP.Mask(connected.Mask), Mask: connected.Mask}
	netOnes, netBits := network.Mask.Size()
	conOnes, conBits := base.Mask.Size()
	if netBits != conBits || conOnes <= netOnes || !network.Contains(base.IP) {
		return false
	}
	return base.Contains(gateway)
}

func anyNet(v6 bool) *net.IPNet {
	if v6 {
		return &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
	}
	return &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
}

// reconcile returns the rules of the source src to delete, the ones of om's
// priority naming another table than table, and whether the rule of the
// table is to add. A rule of another priority is no rule of om's.
func reconcile(existing []netlink.Rule, src *net.IPNet, table int) (del []netlink.Rule, add bool) {
	add = true
	for _, r := range existing {
		if r.Src == nil || r.Src.String() != src.String() || r.Priority != SourceRulePriority {
			continue
		}
		if r.Table == table && add {
			add = false
			continue
		}
		del = append(del, r)
	}
	return del, add
}

// tableNamedByOther returns the rule naming table for another source than
// src, or for no source, as a rule of an ip.rule resource or the om rule of
// an address whose ipv6 hash is the one of src: the table is not the
// address's own, and sharing it would mix the routes of both.
func tableNamedByOther(rules []netlink.Rule, src *net.IPNet, table int) string {
	for _, r := range rules {
		if r.Table != table {
			continue
		}
		if r.Src != nil && r.Src.String() == src.String() {
			continue
		}
		from := "all"
		if r.Src != nil {
			from = r.Src.String()
		}
		return fmt.Sprintf("from %s priority %d", from, r.Priority)
	}
	return ""
}

// Apply sets the rule and the table of the address in the namespace, as
// "ip rule add from <address> lookup <table> priority 32000" and "ip route
// replace ... table <table>" do. A rule of the address to another table, as
// left by a former card, goes with its table.
func (t SourceRoute) Apply(netns ns.NetNS, log *plog.Logger) error {
	if t.Addr == nil || t.Dev == "" || !t.hasTable() {
		return nil
	}
	return netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(t.Dev)
		if err != nil {
			return fmt.Errorf("policy routing of %s: %w", t.Addr.IP, err)
		}
		table := TableID(t.Addr.IP)
		if err := t.checkTableOwner(table, link.Attrs().Index); err != nil {
			return err
		}
		src := t.Src()
		rules, err := netlink.RuleList(t.family())
		if err != nil {
			return err
		}
		if other := tableNamedByOther(rules, src, table); other != "" {
			return fmt.Errorf("policy routing of %s: table %d is named by the rule %s: %w", t.Addr.IP, table, other, ErrSourceTableInUse)
		}
		del, add := reconcile(rules, src, table)
		for _, r := range del {
			t.logIP(log, "rule del from %s lookup %d", src, r.Table)
			if err := netlink.RuleDel(&r); err != nil {
				return err
			}
			if err := flushTable(r.Table, t.family()); err != nil {
				return err
			}
		}
		for _, route := range t.Routes(table, link.Attrs().Index) {
			t.logIP(log, "route replace %s table %d", routeString(route), table)
			if err := netlink.RouteReplace(&route); err != nil {
				return fmt.Errorf("policy routing of %s: route %s: %w", t.Addr.IP, routeString(route), err)
			}
		}
		if add {
			rule := netlink.NewRule()
			rule.Family = t.family()
			rule.Src = src
			rule.Table = table
			rule.Priority = SourceRulePriority
			t.logIP(log, "rule add from %s lookup %d priority %d", src, table, SourceRulePriority)
			if err := netlink.RuleAdd(rule); err != nil {
				return fmt.Errorf("policy routing of %s: rule: %w", t.Addr.IP, err)
			}
		}
		return nil
	})
}

// checkTableOwner refuses a table holding routes of another card than the
// one of the address: a table a user named the number of ours.
func (t SourceRoute) checkTableOwner(table, linkIndex int) error {
	routes, err := netlink.RouteListFiltered(t.family(), &netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	for _, r := range routes {
		if r.LinkIndex != linkIndex {
			return fmt.Errorf("policy routing of %s: table %d: %w", t.Addr.IP, table, ErrSourceTableInUse)
		}
	}
	return nil
}

// Remove deletes the rules of the address of om's priority and the tables
// they name, as "ip rule del" and "ip route flush table" do. The kernel
// removes the routes of a card deleted, but not the rules, and not the routes
// of a card kept, as one shared by several addresses.
func (t SourceRoute) Remove(netns ns.NetNS, log *plog.Logger) error {
	if !t.hasTable() {
		return nil
	}
	return netns.Do(func(_ ns.NetNS) error {
		src := t.Src()
		rules, err := netlink.RuleList(t.family())
		if err != nil {
			return err
		}
		for _, r := range rules {
			if r.Src == nil || r.Src.String() != src.String() || r.Priority != SourceRulePriority {
				continue
			}
			t.logIP(log, "rule del from %s lookup %d", src, r.Table)
			if err := netlink.RuleDel(&r); err != nil {
				return err
			}
			if err := flushTable(r.Table, t.family()); err != nil {
				return err
			}
		}
		return nil
	})
}

// Check returns an error saying what is missing of the policy routing of
// the address: the rule, or the default route of its table.
func (t SourceRoute) Check(netns ns.NetNS) error {
	if t.Addr == nil || t.Dev == "" || !t.hasTable() {
		return nil
	}
	return netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(t.Dev)
		if err != nil {
			return nil
		}
		table := TableID(t.Addr.IP)
		rules, err := netlink.RuleList(t.family())
		if err != nil {
			return err
		}
		if _, add := reconcile(rules, t.Src(), table); add {
			return fmt.Errorf("no policy rule for %s: its replies follow the default route of the namespace", t.Addr.IP)
		}
		want := t.Routes(table, link.Attrs().Index)
		routes, err := netlink.RouteListFiltered(t.family(), &netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
		if err != nil {
			return err
		}
		for _, w := range want {
			if w.Gw == nil {
				continue
			}
			found := false
			for _, r := range routes {
				if r.Gw.Equal(w.Gw) && dstString(r.Dst, t.IPv6()) == dstString(w.Dst, t.IPv6()) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no route %s in the policy table %d of %s", routeString(w), table, t.Addr.IP)
			}
		}
		return nil
	})
}

func flushTable(table, family int) error {
	if table <= maxReservedTable {
		// A table of the kernel, as the main one, is never om's.
		return nil
	}
	routes, err := netlink.RouteListFiltered(family, &netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	for _, r := range routes {
		if err := netlink.RouteDel(&r); err != nil {
			return err
		}
	}
	return nil
}

func (t SourceRoute) logIP(log *plog.Logger, format string, args ...any) {
	if log == nil {
		return
	}
	prefix := "ip "
	if t.IPv6() {
		prefix = "ip -6 "
	}
	log.Infof(prefix+format, args...)
}

func dstString(dst *net.IPNet, v6 bool) string {
	if dst == nil {
		return anyNet(v6).String()
	}
	return dst.String()
}

// routeString writes a route as ip route does.
func routeString(r netlink.Route) string {
	s := dstString(r.Dst, r.Dst != nil && r.Dst.IP.To4() == nil)
	if r.Dst != nil && r.Dst.String() == "0.0.0.0/0" || r.Dst != nil && r.Dst.String() == "::/0" {
		s = "default"
	}
	if r.Gw != nil {
		s += " via " + r.Gw.String()
	}
	if r.Src != nil {
		s += " src " + r.Src.String()
	}
	if r.Flags&int(netlink.FLAG_ONLINK) != 0 {
		s += " onlink"
	}
	return s
}
