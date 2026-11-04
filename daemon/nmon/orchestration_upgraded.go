package nmon

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

// The upgrade of the opensvc packages of the cluster, one node at a time.
//
// Every node is asked by the global expect, and each upgrades in turn: the
// first, in the order of the cluster nodes, of those still expecting it, and
// only when every node of the cluster is live and idle. The node upgrading
// restarts its daemon when opensvc-server is among the packages upgraded, so
// the peers see it leave and come back: the next one waits for it to be back
// and idle, not only for its monitor to say it is done. The upgrade is given
// up when it fails on a node, or when a node is in any failed state: the
// operator has to look at it first.
//
// A daemon restarted by its upgrade starts over with no global expect, and
// adopts the one its peers still carry: it would upgrade again. The id of
// the orchestration is written to a flag file before the upgrade runs, and a
// node finding the id it adopted in the file is one that ran it already: it
// ends its part without running anything. A node that ends its part with the
// file left behind, as the last to upgrade, whose peers ended before it came
// back, removes it once idle with nothing expected.

// upgradeTurn is what upgradeTurn says: the node to upgrade now, or why none
// does yet, or why the upgrade is given up.
type upgradeTurn struct {
	node   string
	wait   string
	failed string
}

// upgradeTurnOf says which node upgrades its packages now: the nodes of the
// cluster, in their order, are all expected live and idle, and the first of
// them still expecting the upgrade is the one. Until then it says what is
// waited for, or what the upgrade is given up on: a node in a failed state,
// the upgrade that failed on it, or anything else.
func upgradeTurnOf(nodes []string, monitors map[string]node.Monitor) upgradeTurn {
	if len(nodes) == 0 {
		for nodename := range monitors {
			nodes = append(nodes, nodename)
		}
		sort.Strings(nodes)
	}
	var t upgradeTurn
	for _, nodename := range nodes {
		mon, ok := monitors[nodename]
		switch {
		case !ok:
			if t.wait == "" {
				t.wait = fmt.Sprintf("node %s is not live", nodename)
			}
		case mon.State.IsOneOf(node.MonitorStatesFailure...):
			if t.failed == "" {
				t.failed = fmt.Sprintf("%s on %s", mon.State, nodename)
			}
		case mon.State != node.MonitorStateIdle:
			if t.wait == "" {
				t.wait = fmt.Sprintf("node %s is %s", nodename, mon.State)
			}
		}
		if t.node == "" && ok && mon.GlobalExpect == node.MonitorGlobalExpectUpgraded {
			t.node = nodename
		}
	}
	switch {
	case t.failed != "":
		return upgradeTurn{failed: t.failed}
	case t.wait != "":
		return upgradeTurn{wait: t.wait}
	case t.node == "":
		return upgradeTurn{wait: "no node expects the upgrade"}
	}
	return upgradeTurn{node: t.node}
}

func (t *Manager) orchestrateUpgraded() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.upgradedFromIdle()
	case node.MonitorStateUpgrade:
	case node.MonitorStateUpgradeSuccess:
		t.upgradedFromUpgradeSuccess()
	case node.MonitorStateUpgradeFailure:
		t.upgradedFromUpgradeFailure()
	default:
		t.log.Warnf("don't know how to orchestrate %s from %s", t.state.GlobalExpect, t.state.State)
	}
}

func (t *Manager) upgradedFromIdle() {
	if t.upgradeFlagIs(t.state.OrchestrationID.String()) {
		t.log.Infof("the packages were upgraded by this orchestration before the daemon restarted")
		t.endUpgraded()
		return
	}
	monitors := make(map[string]node.Monitor, len(t.nodeMonitor)+1)
	maps.Copy(monitors, t.nodeMonitor)
	monitors[t.localhost] = t.state
	turn := upgradeTurnOf(t.clusterConfig.Nodes, monitors)
	switch {
	case turn.failed != "":
		t.log.Errorf("give up the upgrade of the packages: %s", turn.failed)
		t.failUpgraded(turn.failed)
		return
	case turn.wait != "":
		t.logUpgradeWait(turn.wait)
		return
	case turn.node != t.localhost:
		t.logUpgradeWait(fmt.Sprintf("node %s upgrades before this one", turn.node))
		return
	}
	t.upgradeWaiting = ""
	if err := t.writeUpgradeFlag(t.state.OrchestrationID.String()); err != nil {
		t.log.Errorf("give up the upgrade of the packages: %s", err)
		t.failUpgraded(err.Error())
		return
	}
	t.change = true
	t.state.State = node.MonitorStateUpgrade
	t.updateIfChange()
	go func() {
		t.log.Infof("run action update pkg")
		if err := t.crmUpdatePkg(); err != nil {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateUpgrade, newState: node.MonitorStateUpgradeFailure}
		} else {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateUpgrade, newState: node.MonitorStateUpgradeSuccess}
		}
	}()
}

// logUpgradeWait says what the upgrade waits for, when it changes: the
// orchestration is looked at on every event, and the wait is a long one.
func (t *Manager) logUpgradeWait(reason string) {
	if t.upgradeWaiting == reason {
		return
	}
	t.upgradeWaiting = reason
	t.log.Infof("wait to upgrade the packages: %s", reason)
}

// upgradedFromUpgradeSuccess ends the upgrade of a node whose daemon the
// upgrade did not restart: nothing newer, or nothing of the daemon among
// what was upgraded. A restarted daemon ends it from idle, by the flag.
func (t *Manager) upgradedFromUpgradeSuccess() {
	t.log.Infof("the packages are upgraded")
	t.change = true
	t.state.State = node.MonitorStateIdle
	t.endUpgraded()
}

// upgradedFromUpgradeFailure ends the upgrade of this node on its failure,
// which the state keeps saying, and the peers give up on.
func (t *Manager) upgradedFromUpgradeFailure() {
	t.endUpgraded()
}

// failUpgraded ends the part of this node, which ran nothing, on a failure
// elsewhere.
func (t *Manager) failUpgraded(reason string) {
	if t.orchestrationPending != nil {
		t.orchestrationPending.Failed = true
		t.orchestrationPending.Error = reason
	}
	t.endUpgraded()
}

func (t *Manager) endUpgraded() {
	t.removeUpgradeFlag()
	t.change = true
	t.state.GlobalExpect = node.MonitorGlobalExpectNone
	t.clearPending()
	t.endOrchestration()
}

// settleUpgraded is what is left to do with nothing expected: an upgrade
// that ended after its orchestration was aborted leaves the upgraded state,
// and a flag file its orchestration ended without this node, as the last
// one to upgrade, whose peers ended before its daemon came back.
func (t *Manager) settleUpgraded() {
	switch t.state.State {
	case node.MonitorStateUpgradeSuccess:
		t.change = true
		t.state.State = node.MonitorStateIdle
		t.removeUpgradeFlag()
	case node.MonitorStateIdle:
		// Once, not on every event. A daemon just restarted has no time
		// yet: it has not heard its peers, which may still carry the
		// orchestration the flag is of.
		if !t.upgradeFlagSettled && !t.state.GlobalExpectUpdatedAt.IsZero() {
			t.removeUpgradeFlag()
		}
	}
}

// upgradeFlagFile holds the id of the orchestration that upgraded the
// packages of this node, from before the upgrade runs until the node ends
// its part of it: the daemon the upgrade restarts reads there that it did
// run.
func upgradeFlagFile() string {
	return filepath.Join(rawconfig.Paths.Var, "node", "upgrade_orchestration_id")
}

func (t *Manager) writeUpgradeFlag(id string) error {
	if err := os.WriteFile(upgradeFlagFile(), []byte(id+"\n"), 0o644); err != nil {
		return fmt.Errorf("write the upgrade flag: %w", err)
	}
	t.upgradeFlagSettled = false
	return nil
}

func (t *Manager) upgradeFlagIs(id string) bool {
	b, err := os.ReadFile(upgradeFlagFile())
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == id
}

func (t *Manager) removeUpgradeFlag() {
	t.upgradeFlagSettled = true
	if err := os.Remove(upgradeFlagFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.log.Warnf("remove the upgrade flag: %s", err)
	}
}
