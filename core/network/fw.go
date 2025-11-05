//go:build linux

package network

import (
	"fmt"
	"net"
	"reflect"

	"github.com/google/nftables"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/plog"
	"github.com/rs/zerolog"
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
	table = &nftables.Table{
		Family: family,
		Name:   tableName,
	}
	if err := t.Run([]string{"nft", "add", "table", fmtFamily(family), tableName}); err != nil {
		return nil, err
	}
	table, err = t.GetTable(family, tableName)
	if err != nil {
		return nil, err
	}
	t.tables = append(t.tables, table)
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

	s += fmt.Sprintf(" priority %d", chain.Priority)

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
	table, err := h.AddTable(family, "nat")
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
			if err := h.AddRuleSourceJump(cidr); err != nil {
				return err
			}
			if err := h.AddRuleForwardAccept(cidr, dev); err != nil {
				return err
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

func (t *nftHandle) FlushChains() error {
	families := []nftables.TableFamily{
		nftables.TableFamilyIPv4,
		nftables.TableFamilyIPv6,
	}
	chainNames := []struct {
		Table string
		Chain string
	}{
		{"nat", "osvc-masq"},
		{"nat", "osvc-postrouting"},
		{"filter", "osvc-forward"},
	}
	for _, family := range families {
		for _, data := range chainNames {
			if chain, _ := t.GetChain(family, data.Table, data.Chain); chain != nil {
				l := []string{"nft", "flush", "chain", fmtFamily(family), data.Table, data.Chain}
				if err := t.Run(l); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *nftHandle) AddRuleMasq() error {
	families := []nftables.TableFamily{
		nftables.TableFamilyIPv4,
		nftables.TableFamilyIPv6,
	}
	for _, family := range families {
		table, _ := t.GetTable(family, "nat")
		if table != nil {
			if err := t.addRuleMasq(table); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *nftHandle) addRuleMasq(table *nftables.Table) error {
	chain, err := t.AddChain(table, "osvc-masq")
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
	table, err := t.AddTable(family, "nat")
	if err != nil {
		return err
	}
	chain, err := t.AddChain(table, "osvc-masq")
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
	table, err := t.AddTable(family, "nat")
	if err != nil {
		return err
	}
	chain, err := t.AddPostRoutingChain(table, "osvc-postrouting")
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
	table, err := t.AddTable(family, "filter")
	if err != nil {
		return err
	}
	chain, err := t.AddForwardChain(table, "osvc-forward")
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
