//go:build linux

package network

import (
	"fmt"
	"net"
	"strings"

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
	legacyFWChains = []legacyChain{
		{Table: "nat", Chain: fwChainPostrouting},
		{Table: "nat", Chain: fwChainMasq},
		{Table: "filter", Chain: fwChainForward},
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

// fwNetwork is what the ruleset needs to know about a network: the addresses
// it holds, and the device its backend traffic goes through when it has one.
type fwNetwork struct {
	CIDR string
	Dev  string
}

// legacyChain names a chain om left in a table it no longer writes to.
type legacyChain struct {
	Family string
	Table  string
	Chain  string
}

// multicastCIDR is returned from the masquerade chain rather than translated.
// There is no ipv6 counterpart because there never was one.
const multicastCIDR = "224.0.0.0/8"

// fwNetworks returns what the ruleset is rendered from.
func fwNetworks(nws []Networker) []fwNetwork {
	l := make([]fwNetwork, 0, len(nws))
	for _, nw := range nws {
		n := fwNetwork{CIDR: nw.Network()}
		if i, ok := nw.(backendDevNamer); ok {
			n.Dev = i.BackendDevName()
		}
		l = append(l, n)
	}
	return l
}

// fwRuleset renders the nft document a setup applies.
//
// The whole document is one transaction. The table is deleted and defined
// again in full, so the rules are never half there: the kernel swaps the table
// in one step, and a rule nft refuses leaves the ruleset as it was rather than
// partly rebuilt. Adding a rule at a time left the masquerade and the forward
// accepts absent for as long as the setup ran, a quarter of a second of new
// connections leaving a container unmasqueraded.
//
// The "table" line before the "delete" is what makes the delete safe to write
// unconditionally: it creates the table when it is absent, and says nothing
// when it is not.
func fwRuleset(networks []fwNetwork, legacy []legacyChain) (string, error) {
	var sb strings.Builder
	for _, chain := range legacy {
		fmt.Fprintf(&sb, "flush chain %s %s %s\n", chain.Family, chain.Table, chain.Chain)
	}
	for _, chain := range legacy {
		fmt.Fprintf(&sb, "delete chain %s %s %s\n", chain.Family, chain.Table, chain.Chain)
	}
	for _, family := range fwFamilies {
		body, err := fwTable(family, networks)
		if err != nil {
			return "", err
		}
		if body == "" {
			continue
		}
		name := fmtFamily(family)
		fmt.Fprintf(&sb, "table %s %s { }\n", name, fwTableName)
		fmt.Fprintf(&sb, "delete table %s %s\n", name, fwTableName)
		sb.WriteString(body)
	}
	return sb.String(), nil
}

// fwTable renders the table of one address family, or an empty string when no
// network of that family is configured.
func fwTable(family nftables.TableFamily, networks []fwNetwork) (string, error) {
	var returns, jumps, devs []string
	name := fmtFamily(family)
	for _, nw := range networks {
		ip, ipnet, err := net.ParseCIDR(nw.CIDR)
		if err != nil {
			return "", fmt.Errorf("network %s: %w", nw.CIDR, err)
		}
		if ipFamily(ip) != family {
			continue
		}
		returns = append(returns, fmt.Sprintf("\t\t%s daddr %s counter return\n", name, ipnet))
		if nw.Dev == "" {
			continue
		}
		jumps = append(jumps, fmt.Sprintf("\t\t%s saddr %s counter jump %s\n", name, ipnet, fwChainMasq))
		devs = append(devs, nw.Dev)
	}
	if len(returns) == 0 {
		return "", nil
	}
	if family == nftables.TableFamilyIPv4 {
		returns = append(returns, fmt.Sprintf("\t\tip daddr %s counter return\n", multicastCIDR))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "table %s %s {\n", name, fwTableName)

	fmt.Fprintf(&sb, "\tchain %s {\n", fwChainMasq)
	for _, rule := range returns {
		sb.WriteString(rule)
	}
	sb.WriteString("\t\tmasquerade\n\t}\n")

	fmt.Fprintf(&sb, "\tchain %s {\n", fwChainPostrouting)
	sb.WriteString("\t\ttype nat hook postrouting priority srcnat; policy accept;\n")
	for _, rule := range jumps {
		sb.WriteString(rule)
	}
	sb.WriteString("\t}\n")

	fmt.Fprintf(&sb, "\tchain %s {\n", fwChainForward)
	sb.WriteString("\t\ttype filter hook forward priority filter; policy accept;\n")
	for _, dev := range devs {
		fmt.Fprintf(&sb, "\t\tiif \"%s\" counter accept\n", dev)
		fmt.Fprintf(&sb, "\t\toif \"%s\" counter accept\n", dev)
	}
	sb.WriteString("\t}\n}\n")

	return sb.String(), nil
}

// legacyChains returns the chains om left in the tables it no longer writes
// to, so the document deletes the ones this node still has.
//
// They cannot be deleted unconditionally: nft aborts a transaction on a delete
// of an absent chain, and not half applying is the point of the transaction.
func (t *nftHandle) legacyChains() ([]legacyChain, error) {
	l := make([]legacyChain, 0)
	for _, family := range fwFamilies {
		for _, data := range legacyFWChains {
			chain, err := t.GetChain(family, data.Table, data.Chain)
			if err != nil {
				return nil, err
			}
			if chain == nil {
				continue
			}
			l = append(l, legacyChain{Family: fmtFamily(family), Table: data.Table, Chain: data.Chain})
		}
	}
	return l, nil
}

// apply hands the document to nft, which reads a ruleset from stdin and
// applies it as one transaction.
func (t *nftHandle) apply(ruleset string) error {
	if ruleset == "" {
		return nil
	}
	cmd := command.New(
		command.WithName("nft"),
		command.WithVarArgs("-f", "-"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Cmd().Stdin = strings.NewReader(ruleset)
	if t.log != nil {
		t.log.Attr("ruleset", ruleset).Infof("apply the nft ruleset of the om networks")
	}
	return cmd.Run()
}

func setupFW(n logger, nws []Networker) error {
	h := newNFTHandle()
	h.SetLogger(n.Log())
	legacy, err := h.legacyChains()
	if err != nil {
		return err
	}
	ruleset, err := fwRuleset(fwNetworks(nws), legacy)
	if err != nil {
		return err
	}
	return h.apply(ruleset)
}
