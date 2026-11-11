package object

import (
	"slices"

	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of the objects, declared next to them: the
// keywords of the DEFAULT section, of the sections of an object that are not
// resources, as the claims of a nscfg or the stores of a sec, and the ones
// any section accepts, as comment and the pg_* caps.
//
// A group holding the topic of a group of the drivers has its id, name and
// order, as timeouts and scheduling. None has the id of a group every
// resource has: these have no driver, as the groups of the objects, and the
// api tells them apart by their id alone.
//
// No group has strategies. The flex_* keywords are read for a flex object
// only, and stonith acts for a failover one only, but the topology keyword
// alone chooses, as their Depends say: a keyword of the topology not chosen
// is unread, not a second way to configure the object.
var (
	objectGroupIdentity = keywords.Group{
		ID:    "identity",
		Name:  "Identity",
		Text:  keywords.NewText(fs, "text/group/identity"),
		Order: 10,
	}

	objectGroupVolume = keywords.Group{
		ID:    "volume",
		Name:  "Volume",
		Text:  keywords.NewText(fs, "text/group/volume"),
		Order: 12,
	}

	objectGroupSubject = keywords.Group{
		ID:    "subject",
		Name:  "Subject",
		Text:  keywords.NewText(fs, "text/group/subject"),
		Order: 14,
	}

	objectGroupCertificate = keywords.Group{
		ID:    "certificate",
		Name:  "Certificate",
		Text:  keywords.NewText(fs, "text/group/certificate"),
		Order: 16,
	}

	objectGroupGrants = keywords.Group{
		ID:    "grants",
		Name:  "Grants",
		Text:  keywords.NewText(fs, "text/group/grants"),
		Order: 18,
	}

	objectGroupNodes = keywords.Group{
		ID:    "nodes",
		Name:  "Nodes",
		Text:  keywords.NewText(fs, "text/group/nodes"),
		Order: 20,
	}

	objectGroupTopology = keywords.Group{
		ID:    "topology",
		Name:  "Topology",
		Text:  keywords.NewText(fs, "text/group/topology"),
		Order: 22,
	}

	objectGroupPlacement = keywords.Group{
		ID:    "placement",
		Name:  "Placement",
		Text:  keywords.NewText(fs, "text/group/placement"),
		Order: 24,
	}

	objectGroupOrchestration = keywords.Group{
		ID:    "orchestration",
		Name:  "Orchestration",
		Text:  keywords.NewText(fs, "text/group/orchestration"),
		Order: 26,
	}

	objectGroupHA = keywords.Group{
		ID:    "ha",
		Name:  "High availability",
		Text:  keywords.NewText(fs, "text/group/ha"),
		Order: 28,
	}

	objectGroupScheduling = keywords.Group{
		ID:    "scheduling",
		Name:  "Scheduling",
		Text:  keywords.NewText(fs, "text/group/scheduling"),
		Order: 32,
	}

	objectGroupDependencies = keywords.Group{
		ID:    "dependencies",
		Name:  "Dependencies",
		Text:  keywords.NewText(fs, "text/group/dependencies"),
		Order: 34,
	}

	objectGroupSharing = keywords.Group{
		ID:    "sharing",
		Name:  "Sharing",
		Text:  keywords.NewText(fs, "text/group/sharing"),
		Order: 36,
	}

	objectGroupRootless = keywords.Group{
		ID:    "rootless",
		Name:  "Rootless containers",
		Text:  keywords.NewText(fs, "text/group/rootless"),
		Order: 40,
	}

	objectGroupPG = keywords.Group{
		ID:    "pg",
		Name:  "Process group",
		Text:  keywords.NewText(fs, "text/group/pg"),
		Order: 44,
	}

	objectGroupDeployment = keywords.Group{
		ID:    "deployment",
		Name:  "Deployment",
		Text:  keywords.NewText(fs, "text/group/deployment"),
		Order: 50,
	}

	objectGroupActions = keywords.Group{
		ID:    "actions",
		Name:  "Actions",
		Text:  keywords.NewText(fs, "text/group/actions"),
		Order: 60,
	}

	objectGroupClaim = keywords.Group{
		ID:    "claim",
		Name:  "Claim",
		Text:  keywords.NewText(fs, "text/group/claim"),
		Order: 70,
	}

	objectGroupACME = keywords.Group{
		ID:    "acme",
		Name:  "ACME",
		Text:  keywords.NewText(fs, "text/group/acme"),
		Order: 70,
	}

	objectGroupStore = keywords.Group{
		ID:    "store",
		Name:  "Store",
		Text:  keywords.NewText(fs, "text/group/store"),
		Order: 70,
	}

	objectGroupTimeouts = keywords.Group{
		ID:    "timeouts",
		Name:  "Timeouts",
		Text:  keywords.NewText(fs, "text/group/timeouts"),
		Order: 90,
	}

	objectGroups = []keywords.Group{
		objectGroupIdentity,
		objectGroupVolume,
		objectGroupSubject,
		objectGroupCertificate,
		objectGroupGrants,
		objectGroupNodes,
		objectGroupTopology,
		objectGroupPlacement,
		objectGroupOrchestration,
		objectGroupHA,
		objectGroupScheduling,
		objectGroupDependencies,
		objectGroupSharing,
		objectGroupRootless,
		objectGroupPG,
		objectGroupDeployment,
		objectGroupActions,
		objectGroupClaim,
		objectGroupACME,
		objectGroupStore,
		objectGroupTimeouts,
	}
)

// ObjectKeywordGroups returns the groups of the keywords of the objects, the
// ones of a store of an object kind that are not keywords of a driver.
func ObjectKeywordGroups() []keywords.Group {
	return slices.Clone(objectGroups)
}

// objectKeywordGroup returns the group of a keyword of the objects, and
// whether the keyword is one and names a group of the objects.
//
// A keyword of the objects is one of their store, or a copy of it, as the
// one a keyword any section accepts is in the keywords of a driver: it has
// the option, the group and the text of a keyword of the store. A driver may
// name a group of its own by the id of a group of the objects, as timeouts,
// so the id alone does not tell.
func objectKeywordGroup(kw *keywords.Keyword) (keywords.Group, bool) {
	id := kw.GroupID()
	if !slices.ContainsFunc(keywordStore, func(o *keywords.Keyword) bool {
		return o.Option == kw.Option && o.GroupID() == id && o.Text == kw.Text
	}) {
		return keywords.Group{}, false
	}
	for _, g := range objectGroups {
		if g.ID == id {
			return g, true
		}
	}
	return keywords.Group{}, false
}
