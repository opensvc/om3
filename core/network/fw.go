//go:build linux

package network

import (
	"fmt"
	"net"
	"reflect"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/opensvc/om3/util/plog"
)

type (
	nftHandle struct {
		conn     *nftables.Conn
		chains   []*nftables.Chain
		tables   []*nftables.Table
		messages []string
	}
	backendDevNamer interface {
		BackendDevName() string
	}
	logger interface {
		Log() *plog.Logger
	}
)

func newNFTHandle() *nftHandle {
	h := &nftHandle{
		conn: &nftables.Conn{},
	}
	return h
}

func (t *nftHandle) Msgf(format string, v ...interface{}) {
	t.messages = append(t.messages, fmt.Sprintf(format, v...))
}

func (t *nftHandle) Conn() *nftables.Conn {
	return t.conn
}

func (t *nftHandle) Messages() []string {
	return t.messages
}

func (t *nftHandle) Flush() error {
	return t.conn.Flush()
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
	t.Msgf("nft add table %s %s", fmtFamily(family), tableName)
	table = t.conn.AddTable(table)
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
	return t.addChain(chain)
}

func fmtChain(chain *nftables.Chain) string {
	s := fmt.Sprintf("nft add chain %s %s %s {", fmtFamily(chain.Table.Family), chain.Table.Name, chain.Name)
	switch chain.Type {
	case nftables.ChainTypeNAT:
		s += " type nat"
	case nftables.ChainTypeFilter:
		s += " type filter"
	case nftables.ChainTypeRoute:
		s += " type route"
	}
	switch chain.Hooknum {
	case nftables.ChainHookPostrouting:
		s += " hook postrouting"
	case nftables.ChainHookPrerouting:
		s += " hook prerouting"
	case nftables.ChainHookInput:
		s += " hook input"
	case nftables.ChainHookOutput:
		s += " hook output"
	case nftables.ChainHookForward:
		s += " hook forward"
	}
	switch chain.Priority {
	case nftables.ChainPriorityNATSource:
		s += " priority srcnat;"
	case nftables.ChainPriorityNATDest:
		s += " priority dstnat;"
	}
	if chain.Policy != nil {
		switch *chain.Policy {
		case nftables.ChainPolicyAccept:
			s += " policy accept;"
		case nftables.ChainPolicyDrop:
			s += " policy drop;"
		}
	}
	s += " }"
	return s
}

func (t *nftHandle) addChain(chain *nftables.Chain) (*nftables.Chain, error) {
	cachedChain, err := t.GetChain(chain.Table.Family, chain.Table.Name, chain.Name)
	if err != nil {
		return nil, err
	}
	if cachedChain != nil {
		return cachedChain, nil
	}
	chain = t.conn.AddChain(chain)
	s := fmtChain(chain)
	t.Msgf(s)
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
	h.FlushChains()
	for _, other := range nws {
		cidr := other.Network()
		h.AddRuleDestinationReturn(cidr)
		if i, ok := other.(backendDevNamer); ok {
			dev := i.BackendDevName()
			h.AddRuleSourceJump(cidr)
			h.AddRuleForwardAccept(cidr, dev)
		}
	}
	h.AddRuleDestinationReturn("224.0.0.0/8")
	h.AddRuleMasq()
	for _, m := range h.Messages() {
		n.Log().Infof(m)
	}
	return h.Flush()
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

func (t *nftHandle) FlushChains() {
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
				t.Msgf("nft flush chain %s %s %s", fmtFamily(family), data.Table, data.Chain)
				t.conn.FlushChain(chain)
			}
		}
	}
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
	s := fmt.Sprintf("nft add rule %s %s %s", fmtFamily(table.Family), table.Name, chain.Name)
	rule := &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{},
	}
	rule.Exprs = append(rule.Exprs, &expr.Counter{})
	rule.Exprs = append(rule.Exprs, &expr.Masq{})
	s += fmt.Sprintf(" masquerade")
	//printRuleExprs(rule)
	t.conn.AddRule(rule)
	t.Msgf(s)
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
	maskLen := len(ipnet.Mask)
	ones, _ := ipnet.Mask.Size()
	isByteAligned := (ones/8)*8 == ones
	cmpLen := ones / 8
	s := fmt.Sprintf("nft insert rule %s %s %s", fmtFamily(family), table.Name, chain.Name)
	rule := &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{},
	}
	var b []byte
	if ip.To4() == nil {
		rule.Exprs = append(rule.Exprs, &expr.Payload{
			OperationType: expr.PayloadLoad,
			DestRegister:  1,
			Base:          expr.PayloadBaseNetworkHeader,
			Offset:        24,
			Len:           uint32(cmpLen),
		})
		s += " ip6"
		b = ipnet.IP.To16()[0:cmpLen]
	} else {
		rule.Exprs = append(rule.Exprs, &expr.Payload{
			OperationType: expr.PayloadLoad,
			DestRegister:  1,
			Base:          expr.PayloadBaseNetworkHeader,
			Offset:        16,
			Len:           uint32(cmpLen),
		})
		s += " ip"
		b = ipnet.IP.To4()
	}
	if isByteAligned {
		rule.Exprs = append(rule.Exprs, &expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     b[0:cmpLen],
		})
	} else {
		rule.Exprs = append(rule.Exprs, &expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            uint32(maskLen),
			Mask:           ipnet.Mask,
		})
		rule.Exprs = append(rule.Exprs, &expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     b,
		})
	}
	rule.Exprs = append(rule.Exprs, &expr.Counter{})
	rule.Exprs = append(rule.Exprs, &expr.Verdict{
		Kind: expr.VerdictReturn,
	})
	s += fmt.Sprintf(" daddr %s counter return", ipnet.String())
	//printRuleExprs(rule)
	t.conn.InsertRule(rule)
	t.Msgf(s)
	return nil
}

func printRuleExprs(rule *nftables.Rule) {
	fmt.Printf("== Rule %+v\n", rule)
	for _, e := range rule.Exprs {
		fmt.Printf("   %s %+v\n", reflect.TypeOf(e), e)
	}
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
	maskLen := len(ipnet.Mask)
	ones, _ := ipnet.Mask.Size()
	isByteAligned := (ones/8)*8 == ones
	cmpLen := ones / 8
	s := fmt.Sprintf("nft add rule %s %s %s", fmtFamily(family), table.Name, chain.Name)
	rule := &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{},
	}
	var b []byte
	if ip.To4() == nil {
		rule.Exprs = append(rule.Exprs, &expr.Payload{
			OperationType: expr.PayloadLoad,
			DestRegister:  1,
			Base:          expr.PayloadBaseNetworkHeader,
			Offset:        8,
			Len:           uint32(cmpLen),
		})
		s += " ip6"
		b = ipnet.IP.To16()[0:cmpLen]
	} else {
		rule.Exprs = append(rule.Exprs, &expr.Payload{
			OperationType: expr.PayloadLoad,
			DestRegister:  1,
			Base:          expr.PayloadBaseNetworkHeader,
			Offset:        12,
			Len:           uint32(cmpLen),
		})
		s += " ip"
		b = ipnet.IP.To4()
	}
	if isByteAligned {
		rule.Exprs = append(rule.Exprs, &expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     b[0:cmpLen],
		})
	} else {
		rule.Exprs = append(rule.Exprs, &expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            uint32(maskLen),
			Mask:           ipnet.Mask,
		})
		rule.Exprs = append(rule.Exprs, &expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     b,
		})
	}
	rule.Exprs = append(rule.Exprs, &expr.Counter{})
	rule.Exprs = append(rule.Exprs, &expr.Verdict{
		Kind:  expr.VerdictJump,
		Chain: "osvc-masq",
	})
	s += fmt.Sprintf(" saddr %s counter jump osvc-masq", ipnet.String())
	//printRuleExprs(rule)
	t.conn.AddRule(rule)
	t.Msgf(s)
	return nil
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
	b := []byte(dev + "\x00")

	rule := &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyIIFNAME,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     b,
			},
			&expr.Counter{},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	}
	s := fmt.Sprintf("nft add rule %s %s %s iif %s counter accept", fmtFamily(family), table.Name, chain.Name, dev)
	//printRuleExprs(rule)
	t.conn.AddRule(rule)
	t.Msgf(s)

	rule = &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyOIFNAME,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     b,
			},
			&expr.Counter{},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	}
	s = fmt.Sprintf("nft add rule %s %s %s oif %s counter accept", fmtFamily(family), table.Name, chain.Name, dev)
	//printRuleExprs(rule)
	t.conn.AddRule(rule)
	t.Msgf(s)
	return nil
}
