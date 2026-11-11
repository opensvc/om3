package object

import (
	"slices"
	"sync"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of the node and cluster configurations, which
// hold the same keywords, declared once here.
//
// A group is one concept whichever section holds its keywords: the
// endpoint of the collector, of an arbitrator or of an array, the
// credentials of a switch or of a relay. Its id is unique in the two
// configurations, and a group of the concept of a group of the resource
// drivers keeps its id, name and order: identity, addressing, interface,
// routing, scheduling, dns, retention, limits and timeouts.
//
// The sections of a type, as hb, pool, network or array, are the drivers of
// the configuration: a group a section type declares in place of the one of
// its id, for strategies only its keywords follow, is in nodeDriverGroups.
var (
	// nodeGroupIdentity holds what the node, the cluster, or what a
	// section configures is.
	nodeGroupIdentity = keywords.Group{
		ID:    "identity",
		Name:  "Identity",
		Text:  keywords.NewText(fs, "text/group/node/identity"),
		Order: 10,
	}

	// nodeGroupAddressing holds the addresses a network hands out.
	nodeGroupAddressing = keywords.Group{
		ID:    "addressing",
		Name:  "Addressing",
		Text:  keywords.NewText(fs, "text/group/node/addressing"),
		Order: 10,
	}

	// nodeGroupSource holds where a pool takes the storage of its volumes.
	nodeGroupSource = keywords.Group{
		ID:    "source",
		Name:  "Storage source",
		Text:  keywords.NewText(fs, "text/group/node/source"),
		Order: 10,
	}

	// nodeGroupEndpoint holds where the service a section talks to is
	// reached, and whether its certificate is verified.
	nodeGroupEndpoint = keywords.Group{
		ID:    "endpoint",
		Name:  "Endpoint",
		Text:  keywords.NewText(fs, "text/group/node/endpoint"),
		Order: 12,
	}

	// nodeGroupListener holds the addresses and ports the daemon serves on.
	nodeGroupListener = keywords.Group{
		ID:    "listener",
		Name:  "Listener",
		Text:  keywords.NewText(fs, "text/group/node/listener"),
		Order: 12,
	}

	// nodeGroupTransport holds how a heartbeat reaches the other nodes.
	nodeGroupTransport = keywords.Group{
		ID:    "transport",
		Name:  "Transport",
		Text:  keywords.NewText(fs, "text/group/node/transport"),
		Order: 12,
	}

	// nodeGroupNodes holds the nodes the cluster, or a heartbeat, spans.
	nodeGroupNodes = keywords.Group{
		ID:    "nodes",
		Name:  "Nodes",
		Text:  keywords.NewText(fs, "text/group/node/nodes"),
		Order: 14,
	}

	// nodeGroupCredentials holds the secrets authenticating the node to a
	// service, or the nodes to one another.
	nodeGroupCredentials = keywords.Group{
		ID:    "credentials",
		Name:  "Credentials",
		Text:  keywords.NewText(fs, "text/group/node/credentials"),
		Order: 16,
	}

	// nodeGroupInterface holds the device of the node a network is on, and
	// how an address joins it.
	nodeGroupInterface = keywords.Group{
		ID:    "interface",
		Name:  "Interface",
		Text:  keywords.NewText(fs, "text/group/node/interface"),
		Order: 20,
	}

	// nodeGroupCertificates holds the certificates the listener presents,
	// and how it gets and revokes them.
	nodeGroupCertificates = keywords.Group{
		ID:    "certificates",
		Name:  "Certificates",
		Text:  keywords.NewText(fs, "text/group/node/certificates"),
		Order: 20,
	}

	// nodeGroupCreation holds how a pool creates the device of a volume.
	nodeGroupCreation = keywords.Group{
		ID:    "creation",
		Name:  "Creation",
		Text:  keywords.NewText(fs, "text/group/node/creation"),
		Order: 20,
	}

	// nodeGroupHook holds the events a hook runs on, and its command.
	nodeGroupHook = keywords.Group{
		ID:    "hook",
		Name:  "Hook",
		Text:  keywords.NewText(fs, "text/group/node/hook"),
		Order: 20,
	}

	// nodeGroupAuthentication holds who the api accepts, and what it
	// grants them.
	nodeGroupAuthentication = keywords.Group{
		ID:    "authentication",
		Name:  "Authentication and grants",
		Text:  keywords.NewText(fs, "text/group/node/authentication"),
		Order: 22,
	}

	// nodeGroupTools holds the programs of the node a driver runs to
	// manage an array.
	nodeGroupTools = keywords.Group{
		ID:    "tools",
		Name:  "Management tools",
		Text:  keywords.NewText(fs, "text/group/node/tools"),
		Order: 24,
	}

	// nodeGroupFilesystem holds the filesystem a pool formats and mounts
	// its volumes with.
	nodeGroupFilesystem = keywords.Group{
		ID:    "filesystem",
		Name:  "Filesystem",
		Text:  keywords.NewText(fs, "text/group/node/filesystem"),
		Order: 26,
	}

	// nodeGroupRouting holds how the traffic of a network reaches the
	// other nodes and the outside.
	nodeGroupRouting = keywords.Group{
		ID:    "routing",
		Name:  "Routing",
		Text:  keywords.NewText(fs, "text/group/node/routing"),
		Order: 30,
	}

	// nodeGroupVolumes holds what a pool sets in the volumes it makes.
	nodeGroupVolumes = keywords.Group{
		ID:    "volumes",
		Name:  "Volumes",
		Text:  keywords.NewText(fs, "text/group/node/volumes"),
		Order: 30,
	}

	// nodeGroupScheduling holds when the node runs an action on its own.
	nodeGroupScheduling = keywords.Group{
		ID:    "scheduling",
		Name:  "Scheduling",
		Text:  keywords.NewText(fs, "text/group/node/scheduling"),
		Order: 32,
	}

	// nodeGroupQuorum holds what happens when the cluster splits.
	nodeGroupQuorum = keywords.Group{
		ID:    "quorum",
		Name:  "Quorum and split",
		Text:  keywords.NewText(fs, "text/group/node/quorum"),
		Order: 34,
	}

	// nodeGroupDNS holds the cluster nameservers.
	nodeGroupDNS = keywords.Group{
		ID:    "dns",
		Name:  "DNS",
		Text:  keywords.NewText(fs, "text/group/node/dns"),
		Order: 40,
	}

	// nodeGroupEnv holds the environment codes of the node and of the
	// objects it runs.
	nodeGroupEnv = keywords.Group{
		ID:    "env",
		Name:  "Environment code",
		Text:  keywords.NewText(fs, "text/group/node/env"),
		Order: 40,
	}

	// nodeGroupRetention holds what a store keeps.
	nodeGroupRetention = keywords.Group{
		ID:    "retention",
		Name:  "Retention",
		Text:  keywords.NewText(fs, "text/group/node/retention"),
		Order: 40,
	}

	// nodeGroupFeed holds what the node sends the collector.
	nodeGroupFeed = keywords.Group{
		ID:    "feed",
		Name:  "Collector feed",
		Text:  keywords.NewText(fs, "text/group/node/feed"),
		Order: 42,
	}

	// nodeGroupLimits holds the bounds the node holds itself to.
	nodeGroupLimits = keywords.Group{
		ID:    "limits",
		Name:  "Limits",
		Text:  keywords.NewText(fs, "text/group/node/limits"),
		Order: 44,
	}

	// nodeGroupFetch holds the urls the node fetches from, and how.
	nodeGroupFetch = keywords.Group{
		ID:    "fetch",
		Name:  "Fetching urls",
		Text:  keywords.NewText(fs, "text/group/node/fetch"),
		Order: 46,
	}

	// nodeGroupThrottling holds the rate the listener lets requests pass.
	nodeGroupThrottling = keywords.Group{
		ID:    "throttling",
		Name:  "Throttling",
		Text:  keywords.NewText(fs, "text/group/node/throttling"),
		Order: 48,
	}

	// nodeGroupLogging holds what the node logs to syslog.
	nodeGroupLogging = keywords.Group{
		ID:    "logging",
		Name:  "Logging",
		Text:  keywords.NewText(fs, "text/group/node/logging"),
		Order: 50,
	}

	// nodeGroupAsset holds the overrides of the asset the node discovers.
	nodeGroupAsset = keywords.Group{
		ID:    "asset",
		Name:  "Asset",
		Text:  keywords.NewText(fs, "text/group/node/asset"),
		Order: 60,
	}

	// nodeGroupLocation holds where the node is, and who looks after it.
	nodeGroupLocation = keywords.Group{
		ID:    "location",
		Name:  "Location and teams",
		Text:  keywords.NewText(fs, "text/group/node/location"),
		Order: 62,
	}

	// nodeGroupContainers holds the engines and plugins the container
	// resources of the node run with.
	nodeGroupContainers = keywords.Group{
		ID:    "containers",
		Name:  "Containers",
		Text:  keywords.NewText(fs, "text/group/node/containers"),
		Order: 64,
	}

	// nodeGroupRepositories holds where the node downloads its packages
	// and compliance modules from.
	nodeGroupRepositories = keywords.Group{
		ID:    "repositories",
		Name:  "Repositories",
		Text:  keywords.NewText(fs, "text/group/node/repositories"),
		Order: 66,
	}

	// nodeGroupRemote holds how the nodes log in to one another.
	nodeGroupRemote = keywords.Group{
		ID:    "remote",
		Name:  "Remote access",
		Text:  keywords.NewText(fs, "text/group/node/remote"),
		Order: 68,
	}

	// nodeGroupTiming holds the intervals, delays and grace periods of the
	// daemon.
	nodeGroupTiming = keywords.Group{
		ID:    "timing",
		Name:  "Timing",
		Text:  keywords.NewText(fs, "text/group/node/timing"),
		Order: 80,
	}

	// nodeGroupTimeouts holds how long a request may take, and how it is
	// retried.
	nodeGroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/node/timeouts"),
		Order: 90,
	}

	// nodeGroupHP3PARCredentials is the credentials group of a hp3par
	// array: the method keyword says how the driver reaches the array, and
	// each way reads its own credentials, ignoring the other's
	// (drivers/arrayhp3par connectionConfig, drivers/shared/hp3par Command):
	// ssh logs in as username with key, the 3par cli reads the password
	// file pwf.
	//
	// The ssh way is the default method, so no value of the method keyword
	// chooses it: an unset method is ssh, which the validation, reading
	// the values set, could not tell from a When of method=ssh.
	nodeGroupHP3PARCredentials = keywords.Group{
		ID:    nodeGroupCredentials.ID,
		Name:  nodeGroupCredentials.Name,
		Text:  nodeGroupCredentials.Text,
		Order: nodeGroupCredentials.Order,
		Strategies: []keywords.Strategy{
			{
				ID:       "ssh",
				Name:     "Over ssh",
				Text:     keywords.NewText(fs, "text/strategy/node/hp3par/ssh"),
				Uses:     []string{"key", "username"},
				Excludes: []string{"pwf"},
			},
			{
				ID:       "cli",
				Name:     "Through the 3par cli",
				Text:     keywords.NewText(fs, "text/strategy/node/hp3par/cli"),
				Uses:     []string{"pwf"},
				Excludes: []string{"key", "username"},
				When:     keyop.ParseList("method=cli"),
			},
		},
	}

	// nodeGroups are the groups of the node and cluster configurations.
	nodeGroups = []keywords.Group{
		nodeGroupIdentity,
		nodeGroupAddressing,
		nodeGroupSource,
		nodeGroupEndpoint,
		nodeGroupListener,
		nodeGroupTransport,
		nodeGroupNodes,
		nodeGroupCredentials,
		nodeGroupInterface,
		nodeGroupCertificates,
		nodeGroupCreation,
		nodeGroupHook,
		nodeGroupAuthentication,
		nodeGroupTools,
		nodeGroupFilesystem,
		nodeGroupRouting,
		nodeGroupVolumes,
		nodeGroupScheduling,
		nodeGroupQuorum,
		nodeGroupDNS,
		nodeGroupEnv,
		nodeGroupRetention,
		nodeGroupFeed,
		nodeGroupLimits,
		nodeGroupFetch,
		nodeGroupThrottling,
		nodeGroupLogging,
		nodeGroupAsset,
		nodeGroupLocation,
		nodeGroupContainers,
		nodeGroupRepositories,
		nodeGroupRemote,
		nodeGroupTiming,
		nodeGroupTimeouts,
	}

	// nodeDriverGroups are the groups a section type of the node and
	// cluster configurations declares in place of the one of its id, by
	// driver: the keywords of the type naming the id are in the group of
	// the type, the other keywords naming it in the one of the
	// configuration.
	nodeDriverGroups = map[string][]keywords.Group{
		"array.hp3par": {nodeGroupHP3PARCredentials},
	}

	// nodeConfigIndex says which keywords are the ones of the node and
	// cluster configurations, which name the groups of nodeGroups rather
	// than the ones of the resources.
	nodeConfigIndex = sync.OnceValue(func() nodeConfigKeywordIndex {
		idx := nodeConfigKeywordIndex{
			pointers: make(map[*keywords.Keyword]bool),
			sections: make(map[string]bool),
		}
		for _, kw := range slices.Concat(NodeKeywordStore, ccfgKeywordStore) {
			idx.pointers[kw] = true
			switch kw.Section {
			case "":
			case "DEFAULT":
				idx.defaults = append(idx.defaults, kw)
			default:
				idx.sections[kw.Section] = true
			}
		}
		return idx
	})
)

// nodeConfigKeywordIndex indexes the keywords of the node and cluster
// configurations, to tell one of them, or a copy of one, from a keyword of
// an object.
type nodeConfigKeywordIndex struct {
	// pointers are the keywords of the configurations.
	pointers map[*keywords.Keyword]bool

	// sections are the sections of the keywords of the configurations no
	// object configuration has: the keywords of a section of a store of a
	// sec are copies of the ones of the cluster configuration.
	sections map[string]bool

	// defaults are the keywords of the DEFAULT section of the
	// configurations, which the object configurations have too.
	defaults []*keywords.Keyword
}

// has says the keyword is one of the node or cluster configuration, or a
// copy of one, as the keyword definitions of a driver or a section are: of
// one of their sections, or of their DEFAULT section and of the option and
// text of one of theirs. A keyword of any section, as comment, is copied
// with the section it is listed in.
func (t nodeConfigKeywordIndex) has(kw *keywords.Keyword) bool {
	if t.pointers[kw] {
		return true
	}
	switch kw.Section {
	case "":
		return false
	case "DEFAULT":
		return slices.ContainsFunc(t.defaults, func(o *keywords.Keyword) bool {
			return o.Option == kw.Option && o.Text == kw.Text
		})
	default:
		return t.sections[kw.Section]
	}
}

// NodeKeywordGroups returns the groups of the node and cluster
// configurations: the ones of the configurations, then the ones a section
// type declares, each with its driver.
func NodeKeywordGroups() []keywords.DriverGroup {
	l := make([]keywords.DriverGroup, 0, len(nodeGroups))
	for _, g := range nodeGroups {
		l = append(l, keywords.DriverGroup{Group: g})
	}
	for drv, groups := range nodeDriverGroups {
		for _, g := range groups {
			l = append(l, keywords.DriverGroup{Driver: drv, Group: g})
		}
	}
	keywords.SortDriverGroups(l)
	return l
}

// nodeKeywordGroups returns the groups a keyword of the node or cluster
// configuration belongs to, with the driver declaring each: the section
// type of the keyword declaring one of its id, else the configuration. It
// returns false for a keyword of no such configuration, or naming no group
// of them.
func nodeKeywordGroups(kw *keywords.Keyword) ([]keywords.DriverGroup, bool) {
	if !nodeConfigIndex().has(kw) {
		return nil, false
	}
	id := kw.GroupID()
	i := slices.IndexFunc(nodeGroups, func(g keywords.Group) bool { return g.ID == id })
	if i < 0 {
		return nil, false
	}
	l := make([]keywords.DriverGroup, 0, 1)
	shared := len(kw.Types) == 0
	for _, typ := range kw.Types {
		drv := kw.Section + "." + typ
		if g, ok := nodeDriverGroup(drv, id); ok {
			l = append(l, keywords.DriverGroup{Driver: drv, Group: g})
		} else {
			shared = true
		}
	}
	if shared {
		l = append(l, keywords.DriverGroup{Group: nodeGroups[i]})
	}
	return l, true
}

// nodeDriverGroup returns the group of the id a section type declares.
func nodeDriverGroup(drv, id string) (keywords.Group, bool) {
	for _, g := range nodeDriverGroups[drv] {
		if g.ID == id {
			return g, true
		}
	}
	return keywords.Group{}, false
}

// nodeDriverKeywordGroups returns the groups the keywords of a section of
// the driver may name, in a node or cluster configuration: the ones the
// section type declares, and the ones of the configuration. It is the
// referrer hook the validation reads the strategies of a section by.
func nodeDriverKeywordGroups(did driver.ID) []keywords.Group {
	own := nodeDriverGroups[did.String()]
	l := slices.Clone(own)
	for _, g := range nodeGroups {
		if !slices.ContainsFunc(own, func(o keywords.Group) bool { return o.ID == g.ID }) {
			l = append(l, g)
		}
	}
	return l
}

// DriverKeywordGroups returns the groups the keywords of a section of the
// driver may name, the referrer hook the validation of the node
// configuration reads the strategies of a section by.
func (t *Node) DriverKeywordGroups(did driver.ID) []keywords.Group {
	return nodeDriverKeywordGroups(did)
}

// DriverKeywordGroups returns the groups the keywords of a section of the
// driver may name, the referrer hook the validation of the cluster
// configuration reads the strategies of a section by.
func (t *Ccfg) DriverKeywordGroups(did driver.ID) []keywords.Group {
	return nodeDriverKeywordGroups(did)
}
