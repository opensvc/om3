package object

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

func clusterMigrationOf(t *testing.T, config string) Migration {
	t.Helper()
	o, err := newCcfg(naming.Cluster, WithConfigData([]byte(config)), WithVolatile(true))
	require.NoError(t, err)
	return MigrateNodeConfig(o.Config())
}

func deleted(m Migration, section string) bool {
	for _, s := range m.Deletes {
		if s == section {
			return true
		}
	}
	return false
}

func refusedAbout(m Migration, s string) bool {
	for _, refusal := range m.Refusals {
		if strings.Contains(refusal, s) {
			return true
		}
	}
	return false
}

// A keyword written under a former name is written under its name, scope
// included, and the former name is dropped.
func TestNodeMigrationRenamesAliases(t *testing.T) {
	m := clusterMigrationOf(t, `
[node]
min_avail_mem_pct = 10
min_avail_swap_pct@n2 = 5
db_min_ping_interval = 30s

[listener]
tls_port = 1215

[arbitrator#1]
name = arb1.example.com

[stonith#n2]
cmd = /bin/true
`)
	for from, to := range map[string]string{
		"node.min_avail_mem_pct":     "node.min_avail_mem",
		"node.min_avail_swap_pct@n2": "node.min_avail_swap@n2",
		"node.db_min_ping_interval":  "node.collector_ping_interval",
		"listener.tls_port":          "listener.port",
		"arbitrator#1.name":          "arbitrator#1.uri",
		"stonith#n2.cmd":             "stonith#n2.command",
	} {
		_, ok := setOf(m, to)
		assert.True(t, ok, "%s is set", to)
		assert.True(t, unset(m, from), "%s is dropped", from)
	}
	v, _ := setOf(m, "arbitrator#1.uri")
	assert.Equal(t, "arb1.example.com", v)
}

// A former name set beside the name is dropped when the two say the same,
// and kept, with the reason, when they do not.
func TestNodeMigrationAliasBesideItsName(t *testing.T) {
	m := clusterMigrationOf(t, `
[listener]
tls_port = 1215
port = 1215
tls_addr = 10.0.0.1
addr = 10.0.0.2
`)
	assert.True(t, unset(m, "listener.tls_port"))
	assert.False(t, unset(m, "listener.tls_addr"))
	assert.True(t, refusedAbout(m, "listener.tls_addr"))
	assert.Empty(t, m.Sets)
}

// The secret of a pure array names the secret its private_key names.
func TestNodeMigrationPureArraySecret(t *testing.T) {
	m := clusterMigrationOf(t, `
[array#pure1]
type = pure
secret = system/sec/pure1
`)
	v, ok := setOf(m, "array#pure1.private_key")
	require.True(t, ok)
	assert.Equal(t, "system/sec/pure1", v)
	assert.True(t, unset(m, "array#pure1.secret"))
}

// The subnet size of a routed_bridge network is written as a prefix length,
// on the address family of its network, the default included.
func TestNodeMigrationIPsPerNode(t *testing.T) {
	m := clusterMigrationOf(t, `
[network#v4]
type = routed_bridge
network = 10.100.0.0/16
ips_per_node = 250

[network#v6]
type = routed_bridge
network = fd00::/48
ips_per_node = 18446744073709551615

[network#default]
type = routed_bridge
network = 10.200.0.0/16

[network#both]
type = routed_bridge
network = 10.201.0.0/16
ips_per_node = 64
mask_per_node = 24

[network#bridge]
type = bridge
network = 10.202.0.0/16
`)
	for k, want := range map[string]string{
		"network#v4.mask_per_node":      "24",
		"network#v6.mask_per_node":      "64",
		"network#default.mask_per_node": "22",
	} {
		v, ok := setOf(m, k)
		assert.True(t, ok, "%s is set", k)
		assert.Equal(t, want, v, k)
	}
	assert.True(t, unset(m, "network#v4.ips_per_node"))
	assert.False(t, unset(m, "network#default.ips_per_node"), "nothing to drop")
	assert.True(t, unset(m, "network#both.ips_per_node"), "the driver reads mask_per_node first")
	_, ok := setOf(m, "network#both.mask_per_node")
	assert.False(t, ok)
	_, ok = setOf(m, "network#bridge.mask_per_node")
	assert.False(t, ok, "not a routed_bridge")
}

// A network whose address family can not be told is reported.
func TestNodeMigrationIPsPerNodeWithoutNetwork(t *testing.T) {
	m := clusterMigrationOf(t, `
[network#x]
type = routed_bridge
ips_per_node = 250
`)
	assert.Empty(t, m.Sets)
	assert.True(t, refusedAbout(m, "network#x"))
}

// The well-known url of the openid configuration is written as the issuer
// it is the configuration of.
func TestNodeMigrationOpenIDWellKnown(t *testing.T) {
	m := clusterMigrationOf(t, `
[listener]
openid_well_known = https://auth.example.com/realms/r1/.well-known/openid-configuration
`)
	v, ok := setOf(m, "listener.openid_issuer")
	require.True(t, ok)
	assert.Equal(t, "https://auth.example.com/realms/r1", v)
	assert.True(t, unset(m, "listener.openid_well_known"))

	m = clusterMigrationOf(t, `
[listener]
openid_well_known = https://auth.example.com/realms/r1/config
`)
	assert.Empty(t, m.Sets)
	assert.True(t, refusedAbout(m, "listener.openid_well_known"))

	m = clusterMigrationOf(t, `
[listener]
openid_well_known = https://auth.example.com/realms/r1/.well-known/openid-configuration
openid_authority = https://auth.example.com/realms/r1/
`)
	assert.True(t, unset(m, "listener.openid_well_known"), "the issuer, under its alias, says the same")
}

// The schedule of the brocade section moves to the brocade switches setting
// none, and the section goes.
func TestNodeMigrationBrocadeSchedule(t *testing.T) {
	m := clusterMigrationOf(t, `
[brocade]
schedule = @1d

[switch#sw1]
type = brocade
method = ssh

[switch#sw2]
type = brocade
schedule = @2h

[switch#sw3]
type = other
`)
	v, ok := setOf(m, "switch#sw1.schedule")
	require.True(t, ok)
	assert.Equal(t, "@1d", v)
	_, ok = setOf(m, "switch#sw2.schedule")
	assert.False(t, ok, "it has its own")
	_, ok = setOf(m, "switch#sw3.schedule")
	assert.False(t, ok, "not a brocade switch")
	assert.True(t, deleted(m, "brocade"))
}

// What om no longer reads is removed.
func TestNodeMigrationRemovedKeywords(t *testing.T) {
	m := clusterMigrationOf(t, `
[node]
default_mon_format = compact

[reboot]
schedule = @1d

[dequeue_actions]
schedule = @1m

[arbitrator#1]
uri = arb1.example.com
timeout = 5
secret = abc

[hb#1]
type = relay
relay = relay1
username = relayuser
password = system/sec/relay
secret = abc

[stats]
schedule = @10m
`)
	assert.True(t, deleted(m, "reboot"))
	assert.True(t, deleted(m, "dequeue_actions"))
	assert.False(t, deleted(m, "stats"), "the stats section is read")
	for _, k := range []string{"node.default_mon_format", "arbitrator#1.timeout", "arbitrator#1.secret", "hb#1.secret"} {
		assert.True(t, unset(m, k), "%s is dropped", k)
	}
	assert.Empty(t, m.Refusals)
}

// A relay heartbeat still on a secret and a switch reached with telnet are
// reported, and kept.
func TestNodeMigrationReportsDecisions(t *testing.T) {
	m := clusterMigrationOf(t, `
[hb#1]
type = relay
relay = relay1
secret = abc

[switch#sw1]
type = brocade
method = telnet
`)
	assert.True(t, refusedAbout(m, "hb#1"))
	assert.True(t, refusedAbout(m, "switch#sw1"))
	assert.False(t, unset(m, "hb#1.secret"))
}

// The minimum available memory and swap of a v2 node configuration, a
// percentage or a size, are read under their v2 names, and stay as written.
func TestNodeMigrationKeepsTheV2MinAvailForms(t *testing.T) {
	m := clusterMigrationOf(t, `
[node]
min_avail_mem = 2%
min_avail_swap = 2Gi
`)
	assert.Empty(t, m.Sets)
	assert.Empty(t, m.Unsets)
}

// A configuration om reads as it is has nothing to migrate.
func TestNodeMigrationOfACurrentConfiguration(t *testing.T) {
	m := clusterMigrationOf(t, `
[cluster]
name = c1
nodes = n1 n2

[hb#1]
type = unicast

[network#rb]
type = routed_bridge
network = 10.100.0.0/16
mask_per_node = 24
`)
	assert.Empty(t, m.Sets)
	assert.Empty(t, m.Unsets)
	assert.Empty(t, m.Deletes)
	assert.Empty(t, m.Refusals)
}

// The node configuration resolves its keywords as the cluster one does.
func TestNodeMigrationOfANodeConfiguration(t *testing.T) {
	n, err := NewNode(WithConfigData([]byte("[node]\nmin_avail_mem_pct = 10\n")), WithVolatile(true))
	require.NoError(t, err)
	m := MigrateNodeConfig(n.Config())
	v, ok := setOf(m, "node.min_avail_mem")
	require.True(t, ok)
	assert.Equal(t, "10", v)
}

// The rules of each kind of configuration are listed one per paragraph, as
// the help of the migrate commands shows them.
func TestMigrationRulesDoc(t *testing.T) {
	for _, rules := range []MigrationRules{ObjectMigrationRules, NodeMigrationRules} {
		doc := rules.Doc(78)
		assert.Equal(t, len(rules), strings.Count("\n"+doc, "\n  - "), doc)
		for _, line := range strings.Split(doc, "\n") {
			assert.LessOrEqual(t, len(line), 78, line)
		}
	}
	assert.Equal(t, NodeMigrationRules, MigrationRulesOf(naming.KindCcfg))
	assert.Equal(t, ObjectMigrationRules, MigrationRulesOf(naming.KindSvc))
}
