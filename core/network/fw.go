//go:build linux

package network

import (
	"fmt"
	"net"
	"reflect"

	"github.com/google/nftables"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	nftHandle struct {
		conn   *nftables.Conn
		chains []*nftables.Chain
		tables []*nftables.Table
		log    *plog.Logger
	}
	backendDevNamer interface {
		BackendDevName() string
	}
)

// fwTableName is the table om adds its rules to.
//
// They used to go in "nat" and "filter", which are two of the five tables the
// iptables compatibility layer owns. The base chain om added there sits on a
// hook and a priority the standard chain of that table already uses, and that
// makes the whole table unrepresentable in iptables terms: "iptables -t nat -S"
// answers "table `nat' is incompatible, use 'nft' tool". Every firewall driver
// going through iptables is blind to that table from then on. netavark is one
// of them: it stops seeing the chains it created, creates them again, and fails
// with "Chain already exists", so no podman container needing a podman-built
// network starts on a node om has set a network up on.
//
// nftables evaluates the base chains of every table registered on a hook, so om
// keeps its rules by owning a table of its own, and stops standing in the way of
// the tables it does not own.
const (
	fwTableName = "osvc"

	fwChainPostrouting = "osvc-postrouting"
	fwChainMasq        = "osvc-masq"
	fwChainForward     = "osvc-forward"
)

var (
	// fwFamilies are the address families om adds its rules for.
	fwFamilies = []nftables.TableFamily{
		nftables.TableFamilyIPv4,
		nftables.TableFamilyIPv6,
	}

	// legacyFWChains are the chains om used to add to the tables the iptables
	// compatibility layer owns. A setup deletes them, so a node upgrading stops
	// hiding those tables from iptables. The chain holding the jumps comes
	// first: nft refuses to delete a chain another one still jumps to.
	legacyFWChains = []struct {
		Table string
		Chain string
	}{
		{"nat", fwChainPostrouting},
		{"nat", fwChainMasq},
		{"filter", fwChainForward},
	}
)

func newNFTHandle() *nftHandle {
	h := &nftHandle{
		conn: &nftables.Conn{},
	}
	return h
}

func (t *nftHandle) SetLogger(l *plog.Logger) {
	t.log = l
}

func (t *nftHandle) Conn() *nftables.Conn {
	return t.conn
}

func (t *nftHandle) Run(argv []string) error {
	cmd := command.New(
		command.WithName(argv[0]),
		command.WithArgs(argv[1:]),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

// invalidate drops the cached tables and chains, so a lookup made after a
// deletion reads the ruleset again instead of finding what was removed.
func (t *nftHandle) invalidate() {
	t.tables = nil
	t.chains = nil
}

func (t *nftHandle) Tables() ([]*nftables.Table, error) {
	if t.tables != nil {
		return t.tables, nil
	}
	if tables, err := t.conn.ListTables(); err != nil {
		return nil, err
	} else {
		t.tables = tables
	}
	return t.tables, nil
}

func (t *nftHandle) Chains() ([]*nftables.Chain, error) {
	if t.chains != nil {
		return t.chains, nil
	}
	if chains, err := t.conn.ListChains(); err != nil {
		return nil, err
	} else {
		t.chains = chains
	}
	return t.chains, nil
}

func (t *nftHandle) GetTable(family nftables.TableFamily, tableName string) (*nftables.Table, error) {
	tables, err := t.Tables()
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		if table.Name != tableName {
			continue
		}
		if table.Family != family {
			continue
		}
		return table, nil
	}
	return nil, nil
}

func (t *nftHandle) AddTable(family nftables.TableFamily, tableName string) (*nftables.Table, error) {
	table, err := t.GetTable(family, tableName)
	if err != nil {
		return nil, err
	}
	if table != nil {
		return table, nil
	}
	if err := t.Run([]string{"nft", "add", "table", fmtFamily(family), tableName}); err != nil {
		return nil, err
	}
	// The lookup above filled the cache from a ruleset without this table, so
	// read it again rather than answer nil for a table that now exists. The
	// two tables om used to write to were always there, so this never ran.
	t.invalidate()
	table, err = t.GetTable(family, tableName)
	if err != nil {
		return nil, err
	}
	if table == nil {
		return nil, fmt.Errorf("table %s %s is absent right after its creation", fmtFamily(family), tableName)
	}
	return table, nil
}

func (t *nftHandle) GetChain(family nftables.TableFamily, tableName, chainName string) (*nftables.Chain, error) {
	chains, err := t.Chains()
	if err != nil {
		return nil, err
	}
	for _, chain := range chains {
		if chain.Name != chainName {
			continue
		}
		if chain.Table.Name != tableName {
			continue
		}
		if chain.Table.Family != family {
			continue
		}
		return chain, nil
	}
	return nil, nil
}

func (t *nftHandle) AddForwardChain(table *nftables.Table, chainName string) (*nftables.Chain, error) {
	chain := &nftables.Chain{
		Name:     chainName,
		Table:    table,
		Hooknum:  nftables.ChainHookForward,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
	}
	return t.addChain(chain)
}

func (t *nftHandle) AddPostRoutingChain(table *nftables.Table, chainName string) (*nftables.Chain, error) {
	chain := &nftables.Chain{
		Name:     chainName,
		Table:    table,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
		Type:     nftables.ChainTypeNAT,
	}
	return t.addChain(chain)
}

func (t *nftHandle) AddChain(table *nftables.Table, chainName string) (*nftables.Chain, error) {
	chain := &nftables.Chain{
		Name:  chainName,
		Table: table,
	}
	return t.addRegularChain(chain)
}

func fmtRegularChain(chain *nftables.Chain) []string {
	l := []string{"nft", "add", "chain", fmtFamily(chain.Table.Family), chain.Table.Name, chain.Name}
	return l
}

func fmtChain(chain *nftables.Chain) []string {
	l := []string{"nft", "add", "chain", fmtFamily(chain.Table.Family), chain.Table.Name, chain.Name}

	s := "{ type " + string(chain.Type)
	switch chain.Hooknum {
	case nftables.ChainHookPrerouting:
		s += " hook prerouting"
	case nftables.ChainHookInput:
		s += " hook input"
	case nftables.ChainHookForward:
		s += " hook forward"
	case nftables.ChainHookOutput:
		s += " hook output"
	case nftables.ChainHookPostrouting:
		s += " hook postrouting"
	}

	// Priority is a *ChainPriority: formatting the pointer wrote the address
	// of the constant into the rule, and the chain landed on a priority no
	// caller asked for. "priority 4219880" instead of the srcnat 100 put om's
	// masquerade behind every other postrouting chain.
	if chain.Priority != nil {
		s += fmt.Sprintf(" priority %d", *chain.Priority)
	}

	if chain.Policy != nil {
		switch *chain.Policy {
		case nftables.ChainPolicyAccept:
			s += " policy accept"
		case nftables.ChainPolicyDrop:
			s += " policy drop"
		}
	}
	s += "; }"
	return append(l, s)
}

func (t *nftHandle) addRegularChain(chain *nftables.Chain) (*nftables.Chain, error) {
	cachedChain, err := t.GetChain(chain.Table.Family, chain.Table.Name, chain.Name)
	if err != nil {
		return nil, err
	}
	if cachedChain != nil {
		return cachedChain, nil
	}
	l := fmtRegularChain(chain)
	if err := t.Run(l); err != nil {
		return nil, err
	}
	t.chains = append(t.chains, chain)
	return chain, nil
}

func (t *nftHandle) addChain(chain *nftables.Chain) (*nftables.Chain, error) {
	cachedChain, err := t.GetChain(chain.Table.Family, chain.Table.Name, chain.Name)
	if err != nil {
		return nil, err
	}
	if cachedChain != nil {
		return cachedChain, nil
	}
	l := fmtChain(chain)
	if err := t.Run(l); err != nil {
		return nil, err
	}
	t.chains = append(t.chains, chain)
	return chain, nil
}

func debugRules() error {
	h := newNFTHandle()
	family := nftables.TableFamilyIPv4
	table, err := h.AddTable(family, fwTableName)
	if err != nil {
		return err
	}
	chain, err := h.AddChain(table, "osvc-networks")
	if err != nil {
		return err
	}
	rules, err := h.Conn().GetRule(table, chain)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		_ = rule
		fmt.Printf("%+v\n", rule)
		for _, e := range rule.Exprs {
			fmt.Printf(" %s %+v\n", reflect.TypeOf(e), e)
		}
	}
	return nil
}

func setupFW(n logger, nws []Networker) error {
	h := newNFTHandle()
	h.SetLogger(n.Log())
	if err := h.FlushChains(); err != nil {
		return err
	}
	for _, other := range nws {
		cidr := other.Network()
		if err := h.AddRuleDestinationReturn(cidr); err != nil {
			return err
		}
		if i, ok := other.(backendDevNamer); ok {
			dev := i.BackendDevName()
			if dev != "" {
				if err := h.AddRuleSourceJump(cidr); err != nil {
					return err
				}
				if err := h.AddRuleForwardAccept(cidr, dev); err != nil {
					return err
				}
			}
		}
	}
	h.AddRuleDestinationReturn("224.0.0.0/8")
	h.AddRuleMasq()
	return nil
}

func fmtFamily(family nftables.TableFamily) string {
	switch family {
	case nftables.TableFamilyIPv4:
		return "ip"
	case nftables.TableFamilyIPv6:
		return "ip6"
	default:
		return ""
	}
}

func networkFamily(nw Networker) nftables.TableFamily {
	if nw.IsIP6() {
		return nftables.TableFamilyIPv6
	} else {
		return nftables.TableFamilyIPv4
	}
}

func ipFamily(ip net.IP) nftables.TableFamily {
	if ip.To4() == nil {
		return nftables.TableFamilyIPv6
	} else {
		return nftables.TableFamilyIPv4
	}
}

// FlushChains drops what a setup is about to write again.
//
// om's own table is deleted rather than flushed: a chain keeps the hook and the
// priority it was created with, so flushing one an older om created would leave
// it registered where that om put it. The table holds nothing but om's rules,
// and the setup adds them all back.
func (t *nftHandle) FlushChains() error {
	for _, family := range fwFamilies {
		if err := t.deleteTable(family, fwTableName); err != nil {
			return err
		}
		if err := t.deleteLegacyChains(family); err != nil {
			return err
		}
	}
	return nil
}

// deleteTable removes a table and everything in it, when it exists.
func (t *nftHandle) deleteTable(family nftables.TableFamily, tableName string) error {
	table, err := t.GetTable(family, tableName)
	if err != nil {
		return err
	}
	if table == nil {
		return nil
	}
	l := []string{"nft", "delete", "table", fmtFamily(family), tableName}
	if err := t.Run(l); err != nil {
		return err
	}
	t.invalidate()
	return nil
}

// deleteLegacyChains removes the chains om used to add to the nat and filter
// tables, so a node that ran an older om stops hiding them from iptables.
//
// Every chain is flushed before any is deleted: nft refuses to delete a chain a
// rule still jumps to, and the rules doing the jumping live in a chain of this
// same list.
func (t *nftHandle) deleteLegacyChains(family nftables.TableFamily) error {
	found := make([]int, 0, len(legacyFWChains))
	for i, data := range legacyFWChains {
		chain, err := t.GetChain(family, data.Table, data.Chain)
		if err != nil {
			return err
		}
		if chain == nil {
			continue
		}
		l := []string{"nft", "flush", "chain", fmtFamily(family), data.Table, data.Chain}
		if err := t.Run(l); err != nil {
			return err
		}
		found = append(found, i)
	}
	for _, i := range found {
		data := legacyFWChains[i]
		l := []string{"nft", "delete", "chain", fmtFamily(family), data.Table, data.Chain}
		if err := t.Run(l); err != nil {
			return err
		}
	}
	if len(found) > 0 {
		t.invalidate()
	}
	return nil
}

func (t *nftHandle) AddRuleMasq() error {
	for _, family := range fwFamilies {
		table, _ := t.GetTable(family, fwTableName)
		if table != nil {
			if err := t.addRuleMasq(table); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *nftHandle) addRuleMasq(table *nftables.Table) error {
	chain, err := t.AddChain(table, fwChainMasq)
	if err != nil {
		return err
	}
	l := []string{"nft", "add", "rule", fmtFamily(table.Family), table.Name, chain.Name, "masquerade"}
	if err := t.Run(l); err != nil {
		return err
	}
	return nil
}

func (t *nftHandle) AddRuleDestinationReturn(cidr string) error {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	family := ipFamily(ip)
	table, err := t.AddTable(family, fwTableName)
	if err != nil {
		return err
	}
	chain, err := t.AddChain(table, fwChainMasq)
	if err != nil {
		return err
	}
	l := []string{"nft", "insert", "rule", fmtFamily(family), table.Name, chain.Name}
	if ip.To4() == nil {
		l = append(l, "ip6")
	} else {
		l = append(l, "ip")
	}
	l = append(l, "daddr", ipnet.String(), "counter", "return")
	return t.Run(l)
}

func (t *nftHandle) AddRuleSourceJump(cidr string) error {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	family := ipFamily(ip)
	table, err := t.AddTable(family, fwTableName)
	if err != nil {
		return err
	}
	chain, err := t.AddPostRoutingChain(table, fwChainPostrouting)
	if err != nil {
		return err
	}
	l := []string{"nft", "add", "rule", fmtFamily(family), table.Name, chain.Name}
	if ip.To4() == nil {
		l = append(l, "ip6")
	} else {
		l = append(l, "ip")
	}
	l = append(l, "saddr", ipnet.String(), "counter", "jump", "osvc-masq")
	return t.Run(l)
}

func (t *nftHandle) AddRuleForwardAccept(cidr, dev string) error {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	family := ipFamily(ip)
	table, err := t.AddTable(family, fwTableName)
	if err != nil {
		return err
	}
	chain, err := t.AddForwardChain(table, fwChainForward)
	if err != nil {
		return err
	}

	l := []string{"nft", "add", "rule", fmtFamily(family), table.Name, chain.Name, "iif", dev, "counter", "accept"}
	if err := t.Run(l); err != nil {
		return err
	}

	l = []string{"nft", "add", "rule", fmtFamily(family), table.Name, chain.Name, "oif", dev, "counter", "accept"}
	if err := t.Run(l); err != nil {
		return err
	}

	return nil
}
