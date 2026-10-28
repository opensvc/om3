package ccfg

import (
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/stringslice"
)

// onConfigFileUpdated reloads the config parser and emits the updated
// node.Config data in a NodeConfigUpdated event, so other go routines
// can just subscribe to this event to maintain the cache of keywords
// they care about.
func (t *Manager) onConfigFileUpdated(c *msgbus.ConfigFileUpdated) {
	t.pubClusterConfig()
}

func (t *Manager) pubClusterConfig() {
	previousNodes := t.state.Nodes

	state, err := object.SetClusterConfig()
	if err != nil {
		t.log.Errorf("%s", err)
	}
	for _, issue := range state.Issues {
		t.log.Warnf("issue: %s", issue)
	}

	t.state = *state.DeepCopy()
	labelLocalhost := pubsub.Label{"node", t.localhost}

	removed, added := stringslice.Diff(previousNodes, state.Nodes)
	if len(added) > 0 {
		t.log.Tracef("added nodes: %s", added)
	}
	if len(removed) > 0 {
		t.log.Tracef("removed nodes: %s", removed)
	}
	cluster.ConfigData.Set(&state)
	clusternode.Set(state.Nodes)

	t.publisher.Pub(&msgbus.ClusterConfigUpdated{
		Node:           t.localhost,
		Value:          state,
		NodesAdded:     added,
		NodesRemoved:   removed,
		NetworkChanged: t.networkConfigChanged(),
	}, labelLocalhost)

	for _, v := range added {
		t.publisher.Pub(&msgbus.JoinSuccess{AddedNode: v}, labelLocalhost, pubsub.Label{"added_node", v})
	}
	for _, v := range removed {
		t.publisher.Pub(&msgbus.LeaveSuccess{RemovedNode: v}, labelLocalhost, pubsub.Label{"removed_node", v})
	}
}

func (t *Manager) networkConfigChanged() (names []string) {
	clu, err := object.NewCluster()
	if err != nil {
		t.log.Errorf("%s", err)
		return
	}
	cfg := clu.Config()
	names = networkSectionsChanged(t.networkSigs, cfg.SectionStrings(), cfg.SectionSig)
	if len(names) > 0 {
		t.log.Infof("network %v configuration changed", names)
	}
	return
}

// networkSectionsChanged returns the network sections whose signature is not
// the one recorded in sigs, and records it, those added and those removed
// included: a section removed, its signature kept, would be taken for
// unchanged when added back, and its network never set up again.
func networkSectionsChanged(sigs map[string]string, sections []string, sigOf func(string) string) (names []string) {
	present := make(map[string]bool)
	for _, name := range sections {
		if !strings.HasPrefix(name, "network#") && name != "cni" {
			continue
		}
		present[name] = true
		sig := sigOf(name)
		if sig != sigs[name] {
			names = append(names, name)
			sigs[name] = sig
		}
	}
	for name := range sigs {
		if !present[name] {
			names = append(names, name)
			delete(sigs, name)
		}
	}
	slices.Sort(names)
	return
}
