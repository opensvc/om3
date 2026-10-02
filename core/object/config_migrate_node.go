package object

import (
	"fmt"
	"math/bits"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// NodeMigrationRules is the rules the configuration of a node, or of the
// cluster, is migrated by.
var NodeMigrationRules = MigrationRules{
	{
		Doc: "A keyword written under a former name, which om still reads as an alias, " +
			"is written under its name: node.db_min_ping_interval becomes collector_ping_interval, " +
			"node.min_avail_mem_pct becomes min_avail_mem, listener.tls_port becomes port, " +
			"the name of an arbitrator becomes uri, the cmd of a stonith becomes command, " +
			"and so on for every alias the keyword reference lists.",
		apply: migrateAliases,
	},
	{
		Doc:   "The secret of a pure array becomes its private_key, which names the same secret and reads its private_key key.",
		apply: migratePureArraySecret,
	},
	{
		Doc: "The ips_per_node of a routed_bridge network becomes mask_per_node, " +
			"the prefix length of the subnet each node is given, on the address family of its network. " +
			"A network setting neither is given the mask_per_node the ips_per_node default of 1024 amounts to, " +
			"so its subnets keep their size.",
		apply: migrateIPsPerNode,
	},
	{
		Doc: "listener.openid_well_known becomes listener.openid_issuer: " +
			"the url without its /.well-known/openid-configuration end, which om appends itself.",
		apply: migrateOpenIDWellKnown,
	},
	{
		Doc: "The schedule of the brocade section moves to the brocade switch sections that set none, " +
			"as each switch is inventoried on its own schedule, and the brocade section is deleted.",
		apply: migrateBrocadeSchedule,
	},
	{
		Doc: "What om no longer reads is removed: the reboot, rotate_root_pw, stats_collection and dequeue_actions sections, " +
			"node.default_mon_format, the timeout and the secret of an arbitrator, " +
			"and the secret of a relay heartbeat that authenticates with a username.",
		apply: migrateRemovedKeywords,
	},
	{
		Doc: "A relay heartbeat authenticating with a secret, and a switch reached with telnet, are reported and kept: " +
			"what replaces them is a decision, a relay user with the heartbeat grant and a password secret, " +
			"an ssh key or a password secret.",
		apply: reportNodeDecisions,
	},
}

// removedNodeSections is the sections of a node or cluster configuration om
// no longer reads anything of.
var removedNodeSections = []string{"reboot", "rotate_root_pw", "stats_collection", "dequeue_actions"}

// migrateAliases writes the keywords written under a former name under their
// name.
//
// The keyword reference reads the alias as the keyword, so the configuration
// works as it is, but the name it shows is not the one the documentation, the
// completion and the get command know it by.
func migrateAliases(cfg *xconfig.T, m *Migration) {
	if cfg.Referrer == nil {
		return
	}
	for _, section := range cfg.SectionStrings() {
		for _, option := range cfg.Keys(section) {
			k := key.New(section, option)
			kw := cfg.Referrer.KeywordLookup(k, cfg.SectionType(k))
			base, scope := cutScope(option)
			if kw == nil || kw.Option == base || !slices.Contains(kw.Aliases, base) {
				continue
			}
			renameKey(cfg, m, k, withScope(kw.Option, scope))
		}
	}
}

// migratePureArraySecret writes the secret of a pure array as its
// private_key. The secret named the secret holding the key, which the short
// form of private_key names the same way.
func migratePureArraySecret(cfg *xconfig.T, m *Migration) {
	for _, section := range sectionsOf(cfg, "array") {
		if rawValue(cfg, key.New(section, "type")) != "pure" {
			continue
		}
		for _, option := range cfg.Keys(section) {
			if base, scope := cutScope(option); base == "secret" {
				renameKey(cfg, m, key.New(section, option), withScope("private_key", scope))
			}
		}
	}
}

// migrateIPsPerNode writes the subnet size of a routed_bridge network as a
// prefix length.
//
// A number of addresses can not say the size of an ipv6 subnet, which is
// what replaced it. The driver still reads it when mask_per_node is not set,
// and its default when neither is, so a network setting nothing is given the
// prefix that default amounts to: the default goes away with the keyword.
func migrateIPsPerNode(cfg *xconfig.T, m *Migration) {
	for _, section := range sectionsOf(cfg, "network") {
		if rawValue(cfg, key.New(section, "type")) != "routed_bridge" {
			continue
		}
		ipsKey := key.New(section, "ips_per_node")
		maskKey := key.New(section, "mask_per_node")
		hasIPs := cfg.HasKey(ipsKey)
		if cfg.HasKey(maskKey) {
			if hasIPs {
				m.Unsets = append(m.Unsets, ipsKey)
				m.Notes = append(m.Notes, fmt.Sprintf("%s is dropped: %s is set, and the driver reads it first", ipsKey, maskKey))
			}
			continue
		}
		network := rawValue(cfg, key.New(section, "network"))
		_, ipnet, err := net.ParseCIDR(network)
		if err != nil {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s: the address family of network %q can not be told, so neither can the prefix length of its subnets: set %s", section, network, maskKey))
			continue
		}
		_, totalBits := ipnet.Mask.Size()
		ips := uint64(1024)
		said := "the ips_per_node default of 1024"
		if hasIPs {
			ips, err = strconv.ParseUint(rawValue(cfg, ipsKey), 10, 64)
			if err != nil || ips == 0 {
				m.Refusals = append(m.Refusals, fmt.Sprintf("%s: %s = %q is not a number of addresses: set %s", section, ipsKey, rawValue(cfg, ipsKey), maskKey))
				continue
			}
			said = fmt.Sprintf("ips_per_node %d", ips)
		}
		hostBits := bits.Len64(ips - 1)
		if hostBits > totalBits {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s: %d addresses do not fit a %d-bit address: set %s", section, ips, totalBits, maskKey))
			continue
		}
		mask := strconv.Itoa(totalBits - hostBits)
		m.Sets = append(m.Sets, set(section, "mask_per_node", mask))
		if hasIPs {
			m.Unsets = append(m.Unsets, ipsKey)
		}
		m.Notes = append(m.Notes, fmt.Sprintf("%s: %s is mask_per_node %s on network %s", section, said, mask, network))
	}
}

// wellKnownSuffix ends the url of the openid configuration of an issuer.
const wellKnownSuffix = "/.well-known/openid-configuration"

// migrateOpenIDWellKnown writes the url of the openid configuration as the
// issuer it is the configuration of. om derives the url from the issuer, and
// reads the well-known keyword no more.
func migrateOpenIDWellKnown(cfg *xconfig.T, m *Migration) {
	for _, option := range cfg.Keys("listener") {
		base, scope := cutScope(option)
		if base != "openid_well_known" {
			continue
		}
		k := key.New("listener", option)
		value := rawValue(cfg, k)
		issuer, ok := strings.CutSuffix(strings.TrimSuffix(value, "/"), wellKnownSuffix)
		if !ok || issuer == "" {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s = %q is kept: it does not end with %s, so the issuer it is the configuration of can not be told: set listener.openid_issuer", k, value, wellKnownSuffix))
			continue
		}
		target := withScope("openid_issuer", scope)
		if current, isSet := issuerOf(cfg, scope); isSet {
			if strings.TrimSuffix(current, "/") == issuer {
				m.Unsets = append(m.Unsets, k)
				m.Notes = append(m.Notes, fmt.Sprintf("%s is dropped: the issuer is set, to the one it is the configuration of", k))
			} else {
				m.Refusals = append(m.Refusals, fmt.Sprintf("%s = %q is kept: the issuer is set to %q, another one: unset the one you do not mean", k, value, current))
			}
			continue
		}
		m.Sets = append(m.Sets, set("listener", target, issuer))
		m.Unsets = append(m.Unsets, k)
		m.Notes = append(m.Notes, fmt.Sprintf("%s is listener.%s %s now", k, target, issuer))
	}
}

// issuerOf returns the openid issuer the listener sets for a scope, under its
// name or its alias.
func issuerOf(cfg *xconfig.T, scope string) (string, bool) {
	for _, option := range []string{"openid_issuer", "openid_authority"} {
		k := key.New("listener", withScope(option, scope))
		if cfg.HasKey(k) {
			return rawValue(cfg, k), true
		}
	}
	return "", false
}

// migrateBrocadeSchedule moves the schedule of the brocade section, which
// inventoried every brocade switch, to the switches: each is inventoried on
// its own schedule, and nothing reads the brocade section.
func migrateBrocadeSchedule(cfg *xconfig.T, m *Migration) {
	if !cfg.HasSectionString("brocade") {
		return
	}
	switches := make([]string, 0)
	for _, section := range sectionsOf(cfg, "switch") {
		if rawValue(cfg, key.New(section, "type")) == "brocade" {
			switches = append(switches, section)
		}
	}
	for _, option := range cfg.Keys("brocade") {
		base, scope := cutScope(option)
		k := key.New("brocade", option)
		if base != "schedule" {
			m.Notes = append(m.Notes, fmt.Sprintf("%s is dropped: nothing reads the brocade section", k))
			continue
		}
		value := rawValue(cfg, k)
		target := withScope("schedule", scope)
		carried := 0
		for _, section := range switches {
			if cfg.HasKey(key.New(section, target)) {
				continue
			}
			m.Sets = append(m.Sets, set(section, target, value))
			carried++
			m.Notes = append(m.Notes, fmt.Sprintf("%s is %s.%s now", k, section, target))
		}
		if carried == 0 {
			m.Notes = append(m.Notes, fmt.Sprintf("%s is dropped: no brocade switch section without a schedule to carry it to", k))
		}
	}
	m.Deletes = append(m.Deletes, "brocade")
}

// migrateRemovedKeywords removes what om no longer reads, so that a
// validation names what is wrong rather than what is obsolete.
func migrateRemovedKeywords(cfg *xconfig.T, m *Migration) {
	for _, section := range removedNodeSections {
		if cfg.HasSectionString(section) {
			m.Deletes = append(m.Deletes, section)
			m.Notes = append(m.Notes, fmt.Sprintf("the %s section is deleted: om no longer reads it", section))
		}
	}
	unset := func(section string, options ...string) {
		for _, option := range cfg.Keys(section) {
			if base, _ := cutScope(option); slices.Contains(options, base) {
				k := key.New(section, option)
				m.Unsets = append(m.Unsets, k)
				m.Notes = append(m.Notes, fmt.Sprintf("%s is dropped: om no longer reads it", k))
			}
		}
	}
	unset("node", "default_mon_format")
	for _, section := range sectionsOf(cfg, "arbitrator") {
		unset(section, "timeout", "secret")
	}
	for _, section := range relayHeartbeats(cfg) {
		if cfg.HasKey(key.New(section, "username")) {
			unset(section, "secret")
		}
	}
}

// reportNodeDecisions reports what no rule can write, as what replaces it is
// a decision: a heartbeat relay user and its password, a switch credential.
func reportNodeDecisions(cfg *xconfig.T, m *Migration) {
	for _, section := range relayHeartbeats(cfg) {
		if cfg.HasKey(key.New(section, "secret")) && !cfg.HasKey(key.New(section, "username")) {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s: a relay heartbeat authenticates with the username and the password of a user of the relay with the heartbeat grant, and the secret is not read: set %s.username and %s.password", section, section, section))
		}
	}
	for _, section := range sectionsOf(cfg, "switch") {
		if rawValue(cfg, key.New(section, "method")) == "telnet" {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s: telnet sends the password in clear and is refused: set %s.method = ssh, with a key or a password secret", section, section))
		}
	}
}

// relayHeartbeats returns the heartbeat sections of type relay.
func relayHeartbeats(cfg *xconfig.T) []string {
	l := make([]string, 0)
	for _, section := range sectionsOf(cfg, "hb") {
		if rawValue(cfg, key.New(section, "type")) == "relay" {
			l = append(l, section)
		}
	}
	return l
}

// renameKey writes the value of a key under another option of its section,
// and drops it. A key whose new option is set too is dropped only when the
// two say the same.
func renameKey(cfg *xconfig.T, m *Migration, from key.T, to string) {
	value := rawValue(cfg, from)
	toKey := key.New(from.Section, to)
	if cfg.HasKey(toKey) {
		if rawValue(cfg, toKey) == value {
			m.Unsets = append(m.Unsets, from)
			m.Notes = append(m.Notes, fmt.Sprintf("%s is dropped: %s says the same", from, toKey))
		} else {
			m.Refusals = append(m.Refusals, fmt.Sprintf("%s is kept: %s is set too, to another value: unset the one you do not mean", from, toKey))
		}
		return
	}
	m.Sets = append(m.Sets, set(from.Section, to, value))
	m.Unsets = append(m.Unsets, from)
	m.Notes = append(m.Notes, fmt.Sprintf("%s is %s now", from, toKey))
}

// sectionsOf returns the sections of a group, as "hb" for "hb#1" and "hb#2".
func sectionsOf(cfg *xconfig.T, group string) []string {
	l := make([]string, 0)
	for _, section := range cfg.SectionStrings() {
		if section == group || strings.HasPrefix(section, group+"#") {
			l = append(l, section)
		}
	}
	return l
}

// rawValue returns the value of a key as written, unevaluated.
func rawValue(cfg *xconfig.T, k key.T) string {
	v, _ := cfg.GetStrict(k)
	return v
}

// withScope returns the option written for the node scope names, or the
// option itself when scope is empty.
func withScope(option, scope string) string {
	if scope == "" {
		return option
	}
	return option + "@" + scope
}
