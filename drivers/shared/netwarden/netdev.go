package netwarden

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/netrule"
)

type (
	// Netdev is the filter of the network interfaces of a guest: the
	// backend of the resources whose traffic no cgroup carries, as a kvm
	// guest, whose packets leave qemu through tap interfaces.
	//
	// Each interface has a nftables table of the netdev family of its
	// own, holding the filter as named sets, a pair by rule, one of each
	// address family, and two chains reading them in the order of the
	// rules: one on the ingress hook of the interface, which sees what
	// the guest sends, judged by its destination, and one on its egress
	// hook, which sees what is sent to the guest, judged by its source.
	// So the filter holds in both directions, as the one of a cgroup
	// does, with no connection tracking, which the netdev family has
	// none of.
	//
	// A chain is a line by rule and by family, which accepts or drops
	// the packets whose address is in the set of the rule, on the ports
	// of the rule when it names some: the first line matching decides,
	// as the first rule does. So the chains are the list of the rules,
	// and the sets what they resolve to: the addresses the rules name
	// moving under a running guest replace the elements of the sets, and
	// a list of rules changed replaces the table whole, sets and chains,
	// each in one transaction, which nftables applies whole or not at
	// all.
	Netdev struct {
		// Devs are the interfaces, which the driver creates before the
		// guest is started.
		Devs []string
	}

	// nftRunner runs nft, and is replaced by the tests.
	nftRunner interface {
		// Load has nft read a script, as one transaction.
		Load(ctx context.Context, script string) error

		// List returns the table named of the netdev family, as json,
		// and errNoTable when there is none.
		List(ctx context.Context, table string) ([]byte, error)
	}

	nftExec struct{}
)

const (
	// netdevTablePrefix starts the name of the table of an interface.
	netdevTablePrefix = "osvc_netwarden_"

	// netdevPriority places the chains after the ones keeping a guest
	// to its own addresses, at -500: what those drop is not judged
	// here, and what they accept is.
	netdevPriority = -400

	// nftTimeout bounds a run of nft.
	nftTimeout = 20 * time.Second
)

var (
	errNoTable = errors.New("no such table")

	// nft runs nft, and is replaced by the tests.
	nft nftRunner = nftExec{}

	// hasNft says the node has nft, and is replaced by the tests.
	hasNft = func() bool {
		_, err := exec.LookPath("nft")
		return err == nil
	}

	// hasDev says the node has the network interface named, and is
	// replaced by the tests.
	hasDev = func(name string) bool {
		_, err := net.InterfaceByName(name)
		return err == nil
	}

	// kernelRelease returns the release of the running kernel, and is
	// replaced by the tests.
	kernelRelease = func() string {
		b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
		return strings.TrimSpace(string(b))
	}

	// netdevInfra are the lines of each chain passing what an interface
	// needs to hold an address, before the rules, see netdevChain.
	netdevInfra = map[bool][]string{
		false: {
			"ether type arp accept",
			"ether type ip udp sport 68 udp dport 67 accept",
			"ether type ip6 icmpv6 type { nd-router-solicit, nd-neighbor-solicit, nd-neighbor-advert, mld-listener-query, mld-listener-report, mld-listener-done, mld2-listener-report } accept",
			"ether type ip6 udp sport 546 udp dport 547 accept",
		},
		true: {
			"ether type arp accept",
			"ether type ip udp sport 67 udp dport 68 accept",
			"ether type ip icmp type { destination-unreachable, time-exceeded, parameter-problem } accept",
			"ether type ip6 icmpv6 type { destination-unreachable, packet-too-big, time-exceeded, parameter-problem, nd-router-advert, nd-neighbor-solicit, nd-neighbor-advert, mld-listener-query, mld-listener-report, mld-listener-done, mld2-listener-report } accept",
			"ether type ip6 udp sport 547 udp dport 546 accept",
		},
	}

	// netdevChains are the two chains of a table: the name of each, and
	// whether it judges what is sent to the guest.
	netdevChains = []struct {
		name    string
		toGuest bool
	}{{"from_guest", false}, {"to_guest", true}}
)

// netdevSet is a set of a table: the addresses of a family a step of the
// filter names.
type netdevSet struct {
	name string

	// family is 4 or 6, and step the place of the step in the filter.
	family int
	step   int

	// elements are the ranges, as nft reads them.
	elements []string
}

// netdevSetName returns the name of the set of the family given of the
// step at the place given: s2_deny_4, and s3_allow_tcp_5432_5432_6 for a
// step on ports.
//
// The name says all of the step but its addresses, so the sets of a table
// tell the list of rules its chains were written for: a table is read back
// as a filter from its sets, and one whose sets are not the ones of the
// filter to set is one whose chains are to be written again.
func netdevSetName(i int, step netrule.Step, family int) string {
	if step.HasPort() {
		return fmt.Sprintf("s%d_%s_%s_%d_%d_%d", i, step.Action, step.Proto, step.Min, step.Max, family)
	}
	return fmt.Sprintf("s%d_%s_%d", i, step.Action, family)
}

// parseNetdevSetName reads the place, the step and the family the name of
// a set says.
func parseNetdevSetName(name string) (i int, step netrule.Step, family int, err error) {
	words := strings.Split(name, "_")
	bad := func() (int, netrule.Step, int, error) {
		return 0, netrule.Step{}, 0, fmt.Errorf("the set %s is none of a filter om loaded", name)
	}
	if (len(words) != 3 && len(words) != 6) || !strings.HasPrefix(words[0], "s") {
		return bad()
	}
	if i, err = strconv.Atoi(words[0][1:]); err != nil || i < 0 {
		return bad()
	}
	step.Action = netrule.Action(words[1])
	if step.Action != netrule.ActionAllow && step.Action != netrule.ActionDeny {
		return bad()
	}
	if len(words) == 6 {
		lo, errLo := strconv.ParseUint(words[3], 10, 16)
		hi, errHi := strconv.ParseUint(words[4], 10, 16)
		if (words[2] != "tcp" && words[2] != "udp") || errLo != nil || errHi != nil || lo == 0 || hi < lo {
			return bad()
		}
		step.Proto, step.Min, step.Max = words[2], uint16(lo), uint16(hi)
	}
	switch words[len(words)-1] {
	case "4":
		family = 4
	case "6":
		family = 6
	default:
		return bad()
	}
	return i, step, family, nil
}

// netdevSets returns the sets of a filter: two by step, of the ipv4 then of
// the ipv6 addresses, in the order of the steps.
func netdevSets(filter netrule.Filter) []netdevSet {
	l := make([]netdevSet, 0, 2*len(filter.Steps))
	for i, step := range filter.Steps {
		v4 := netdevSet{name: netdevSetName(i, step, 4), family: 4, step: i}
		v6 := netdevSet{name: netdevSetName(i, step, 6), family: 6, step: i}
		for _, prefix := range netrule.Normalize(step.Ranges) {
			if prefix.Addr().Is4() {
				v4.elements = append(v4.elements, prefix.String())
			} else {
				v6.elements = append(v6.elements, prefix.String())
			}
		}
		l = append(l, v4, v6)
	}
	return l
}

func (nftExec) Load(ctx context.Context, script string) error {
	ctx, cancel := context.WithTimeout(ctx, nftTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft -f: %w: %s", err, bytes.TrimSpace(b))
	}
	return nil
}

func (nftExec) List(ctx context.Context, table string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, nftTimeout)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "nft", "-j", "list", "table", "netdev", table)
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "No such file or directory") {
			return nil, errNoTable
		}
		return nil, fmt.Errorf("nft list table netdev %s: %w: %s", table, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return b, nil
}

// NetdevTable returns the name of the table of an interface.
func NetdevTable(dev string) string {
	return netdevTablePrefix + dev
}

// egressSupported says the kernel of the release given has the egress hook
// of the netdev family, which Linux 5.16 brought. A release not read as one
// is taken for a kernel having it: nft refuses the chain where it does not.
func egressSupported(release string) bool {
	major, rest, ok := strings.Cut(release, ".")
	if !ok {
		return true
	}
	digits := func(s string) string {
		for i, c := range s {
			if c < '0' || c > '9' {
				return s[:i]
			}
		}
		return s
	}
	a, err := strconv.Atoi(major)
	if err != nil {
		return true
	}
	b, err := strconv.Atoi(digits(rest))
	if err != nil {
		return true
	}
	return a > 5 || (a == 5 && b >= 16)
}

// Check says why no filter can be applied on the interfaces, before any is
// created: nil, or the reason a start is refused, which would run the guest
// unfiltered.
//
// What is sent to a guest is seen by the egress hook of its interface
// alone: a kernel without it would filter what the guest sends, and nothing
// of what reaches it.
func (t Netdev) Check() error {
	if len(t.Devs) == 0 {
		return fmt.Errorf("the resource has no network interface made by om to filter")
	}
	if !hasNft() {
		return fmt.Errorf("the filter of a network interface is set by nft, which this node has not")
	}
	if release := kernelRelease(); !egressSupported(release) {
		return fmt.Errorf("the kernel %s has no egress hook on a network interface, which Linux 5.16 brought: what is sent to the guest would not be filtered", release)
	}
	return nil
}

// netdevChain returns the lines of a chain of a table, for the filter
// given: the one of what the guest sends, judged by its destination, or of
// what is sent to it, judged by its source.
//
// What an interface needs to hold an address is passed before the rules,
// as it is no exchange with a host a rule could name: arp, the neighbour
// discovery of ipv6, and the dhcp exchange of a client. So are, towards the
// guest, the icmp errors, which tell it of a path and come from routers no
// rule names, as they reach the processes of a filtered cgroup. Anything
// else that is neither ipv4 nor ipv6, as a frame carrying a vlan tag, is
// dropped: the sets would not see the addresses in it.
//
// Then the steps of the filter, in their order, a line by family: the
// first one whose set holds the address of the packet accepts it, or drops
// it. A step on ports matches the port of the address it names, the
// destination port of what the guest sends and the source port of what it
// is sent. A packet with no port to read, as the later fragments of a
// datagram, matches no step on ports.
//
// A packet no step matches passes, as netrule.Filter says: a filter made of
// rules ends with the step of the implicit deny any, which leaves none.
func netdevChain(dev string, toGuest bool, filter netrule.Filter) []string {
	name, hook, addr, port := "from_guest", "ingress", "daddr", "dport"
	if toGuest {
		name, hook, addr, port = "to_guest", "egress", "saddr", "sport"
	}
	l := []string{
		fmt.Sprintf("  chain %s {", name),
		fmt.Sprintf("    type filter hook %s device %q priority %d; policy drop;", hook, dev, netdevPriority),
	}
	for _, s := range netdevInfra[toGuest] {
		l = append(l, "    "+s)
	}
	for i, step := range filter.Steps {
		verdict := "drop"
		if step.Action == netrule.ActionAllow {
			verdict = "accept"
		}
		ports := ""
		if step.HasPort() {
			ports = fmt.Sprintf(" %s %s %d", step.Proto, port, step.Min)
			if step.Max != step.Min {
				ports += "-" + strconv.Itoa(int(step.Max))
			}
		}
		l = append(l,
			fmt.Sprintf("    ip %s @%s%s %s", addr, netdevSetName(i, step, 4), ports, verdict),
			fmt.Sprintf("    ip6 %s @%s%s %s", addr, netdevSetName(i, step, 6), ports, verdict),
		)
	}
	l = append(l,
		"    ether type { ip, ip6 } accept",
		"  }",
	)
	return l
}

// netdevChainRules returns the number of rules of a chain of a filter of
// the number of steps given: what a table listed is checked against.
func netdevChainRules(toGuest bool, steps int) int {
	return len(netdevInfra[toGuest]) + 2*steps + 1
}

// NetdevRuleset returns what nft loads to filter an interface from nothing,
// or under another list of rules: its table, replacing the one there, with
// the sets of the filter and the two chains reading them.
func NetdevRuleset(dev string, filter netrule.Filter) string {
	table := NetdevTable(dev)
	l := []string{
		// A table can only be deleted where there is one.
		fmt.Sprintf("add table netdev %s", table),
		fmt.Sprintf("delete table netdev %s", table),
		fmt.Sprintf("table netdev %s {", table),
	}
	for _, set := range netdevSets(filter) {
		l = append(l,
			fmt.Sprintf("  set %s {", set.name),
			fmt.Sprintf("    type ipv%d_addr", set.family),
			"    flags interval",
		)
		if len(set.elements) > 0 {
			l = append(l, fmt.Sprintf("    elements = { %s }", strings.Join(set.elements, ", ")))
		}
		l = append(l, "  }")
	}
	for _, chain := range netdevChains {
		l = append(l, netdevChain(dev, chain.toGuest, filter)...)
	}
	l = append(l, "}")
	return strings.Join(l, "\n") + "\n"
}

// NetdevUpdate returns what nft loads to replace the addresses of the
// filter of an interface already filtered by the same list of rules: the
// elements of its sets, the chains left as they are.
func NetdevUpdate(dev string, filter netrule.Filter) string {
	table := NetdevTable(dev)
	sets := netdevSets(filter)
	l := make([]string, 0, 2*len(sets))
	for _, set := range sets {
		l = append(l, fmt.Sprintf("flush set netdev %s %s", table, set.name))
	}
	for _, set := range sets {
		if len(set.elements) > 0 {
			l = append(l, fmt.Sprintf("add element netdev %s %s { %s }", table, set.name, strings.Join(set.elements, ", ")))
		}
	}
	return strings.Join(l, "\n") + "\n"
}

// sameRules says the two filters are the steps of the same list of rules,
// whatever the addresses they resolve to: the same verbs, on the same
// ports, in the same order, so the same sets and the same chains.
func sameRules(a, b netrule.Filter) bool {
	if len(a.Steps) != len(b.Steps) {
		return false
	}
	for i := range a.Steps {
		if netdevSetName(i, a.Steps[i], 4) != netdevSetName(i, b.Steps[i], 4) {
			return false
		}
	}
	return true
}

// Load filters the interfaces from nothing, as a start does once it made
// them and before it plugs them: their tables are loaded whole, in one
// transaction. A filter filtering nothing removes the tables.
func (t Netdev) Load(filter netrule.Filter) error {
	if filter.IsZero() {
		return t.remove()
	}
	var script strings.Builder
	for _, dev := range t.Devs {
		script.WriteString(NetdevRuleset(dev, filter))
	}
	return nft.Load(context.Background(), script.String())
}

// Update replaces the filter of the interfaces of a running guest, in one
// transaction, so no packet is judged by a filter half replaced.
//
// An interface whose table is the one of the same list of rules has the
// elements of its sets replaced: the addresses moved, the chains are the
// same. Any other is loaded whole in the same transaction, sets and chains:
// one whose rules changed, whose chains are another list, one with no
// table, as one given rules while its guest runs, and one whose table does
// not read back whole, a chain or a set taken away, where replacing the
// elements alone would leave the traffic the missing chain judged
// unfiltered.
func (t Netdev) Update(filter netrule.Filter) error {
	if filter.IsZero() {
		return t.remove()
	}
	var script strings.Builder
	for _, dev := range t.Devs {
		b, err := nft.List(context.Background(), NetdevTable(dev))
		switch {
		case errors.Is(err, errNoTable):
			script.WriteString(NetdevRuleset(dev, filter))
		case err != nil:
			return err
		default:
			if current, err := parseNetdevTable(b); err != nil || !sameRules(current, filter) {
				script.WriteString(NetdevRuleset(dev, filter))
			} else {
				script.WriteString(NetdevUpdate(dev, filter))
			}
		}
	}
	return nft.Load(context.Background(), script.String())
}

// remove removes the tables of the interfaces, the ones there are.
func (t Netdev) remove() error {
	var script strings.Builder
	for _, dev := range t.Devs {
		if _, err := nft.List(context.Background(), NetdevTable(dev)); err == nil {
			fmt.Fprintf(&script, "delete table netdev %s\n", NetdevTable(dev))
		}
	}
	if script.Len() == 0 {
		return nil
	}
	return nft.Load(context.Background(), script.String())
}

// HasFilter says an interface has a table.
func (t Netdev) HasFilter() bool {
	for _, dev := range t.Devs {
		if _, err := nft.List(context.Background(), NetdevTable(dev)); err == nil {
			return true
		}
	}
	return false
}

// HasMembers says an interface is there: a guest may run behind it.
func (t Netdev) HasMembers() bool {
	for _, dev := range t.Devs {
		if hasDev(dev) {
			return true
		}
	}
	return false
}

// Current returns the filter the interfaces hold, read from the sets of
// their tables, and an error when they do not hold one and the same: an
// interface with no table or missing a chain, or whose sets differ from
// the ones of the first.
func (t Netdev) Current() (netrule.Filter, error) {
	var current netrule.Filter
	for i, dev := range t.Devs {
		b, err := nft.List(context.Background(), NetdevTable(dev))
		if errors.Is(err, errNoTable) {
			return current, fmt.Errorf("the interface %s has no filter: no nft table %s", dev, NetdevTable(dev))
		}
		if err != nil {
			return current, err
		}
		filter, err := parseNetdevTable(b)
		if err != nil {
			return current, fmt.Errorf("the nft table %s of the interface %s: %w", NetdevTable(dev), dev, err)
		}
		if i == 0 {
			current = filter
			continue
		}
		if !filter.Equal(current) {
			return current, fmt.Errorf("the interfaces %s and %s do not hold the same filter", t.Devs[0], dev)
		}
	}
	return current, nil
}

// parseNetdevTable reads the filter of a table from what nft lists of it:
// a step by pair of sets, as their names say them, with the addresses they
// hold. It refuses a table that is not one om loaded whole: missing one of
// the two chains, or a set of a step, holding a set of another name, or
// whose chains do not hold the rules of its steps, by their number.
func parseNetdevTable(b []byte) (netrule.Filter, error) {
	var filter netrule.Filter
	var listing struct {
		Nftables []struct {
			Chain *struct {
				Name string `json:"name"`
				Hook string `json:"hook"`
			} `json:"chain"`
			Rule *struct {
				Chain string `json:"chain"`
			} `json:"rule"`
			Set *struct {
				Name string            `json:"name"`
				Elem []json.RawMessage `json:"elem"`
			} `json:"set"`
		} `json:"nftables"`
	}
	if err := json.Unmarshal(b, &listing); err != nil {
		return filter, err
	}
	hooks := make(map[string]string)
	rules := make(map[string]int)
	type family struct {
		step    netrule.Step
		present bool
	}
	steps := make(map[int]*[2]family)
	for _, o := range listing.Nftables {
		if o.Chain != nil {
			hooks[o.Chain.Hook] = o.Chain.Name
		}
		if o.Rule != nil {
			rules[o.Rule.Chain]++
		}
		if o.Set == nil {
			continue
		}
		i, step, fam, err := parseNetdevSetName(o.Set.Name)
		if err != nil {
			return filter, err
		}
		for _, raw := range o.Set.Elem {
			prefix, err := parseNftPrefix(raw)
			if err != nil {
				return filter, fmt.Errorf("set %s: %w", o.Set.Name, err)
			}
			step.Ranges = append(step.Ranges, prefix)
		}
		if steps[i] == nil {
			steps[i] = &[2]family{}
		}
		steps[i][fam/6] = family{step: step, present: true}
	}
	for i := 0; i < len(steps); i++ {
		pair, ok := steps[i]
		if !ok {
			return filter, fmt.Errorf("no set of the rule %d", i+1)
		}
		v4, v6 := pair[0], pair[1]
		if !v4.present || !v6.present {
			return filter, fmt.Errorf("a set of the rule %d is missing", i+1)
		}
		if netdevSetName(i, v4.step, 4) != netdevSetName(i, v6.step, 4) {
			return filter, fmt.Errorf("the two sets of the rule %d are not of the same rule", i+1)
		}
		step := v4.step
		step.Ranges = netrule.Normalize(append(step.Ranges, v6.step.Ranges...))
		filter.Steps = append(filter.Steps, step)
	}
	for _, hook := range []string{"ingress", "egress"} {
		if _, ok := hooks[hook]; !ok {
			return filter, fmt.Errorf("no chain on the %s hook", hook)
		}
	}
	for _, chain := range netdevChains {
		name := hooks["ingress"]
		if chain.toGuest {
			name = hooks["egress"]
		}
		if want := netdevChainRules(chain.toGuest, len(filter.Steps)); rules[name] != want {
			return filter, fmt.Errorf("the chain %s holds %d rules, not the %d of its %d steps", name, rules[name], want, len(filter.Steps))
		}
	}
	return filter, nil
}

// parseNftPrefix reads an address or a range of a set, as nft lists the ones
// it was given as a range with a length: a range between two addresses is
// none om gave it.
func parseNftPrefix(raw json.RawMessage) (netip.Prefix, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, err
		}
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	var o struct {
		Prefix *struct {
			Addr string `json:"addr"`
			Len  int    `json:"len"`
		} `json:"prefix"`
	}
	if err := json.Unmarshal(raw, &o); err != nil || o.Prefix == nil {
		return netip.Prefix{}, fmt.Errorf("the element %s is neither an address nor a range with a length", raw)
	}
	addr, err := netip.ParseAddr(o.Prefix.Addr)
	if err != nil {
		return netip.Prefix{}, err
	}
	return addr.Prefix(o.Prefix.Len)
}
