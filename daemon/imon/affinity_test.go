package imon

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// newAffinityTestManager returns the manager of a failover object down on
// node1, node2 and node3, placed in that order, its instances able to start
// on every node but for the affinities given.
func newAffinityTestManager(cfg instance.ActorConfig) *Manager {
	m := newTestManager(&pubSpy{})
	m.instConfig.ActorConfig = &cfg
	m.objStatus.PlacementPolicy = placement.NodesOrder
	m.scopeNodes = []string{"node1", "node2", "node3"}
	m.nodeStatus = make(map[string]node.Status)
	m.nodeMonitor = make(map[string]node.Monitor)
	for _, n := range m.scopeNodes {
		m.nodeStatus[n] = node.Status{}
		m.nodeMonitor[n] = node.Monitor{State: node.MonitorStateIdle}
		m.instStatus[n] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
		m.instMonitor[n] = instance.Monitor{State: instance.MonitorStateIdle}
	}
	return m
}

// setAffine sets the avail status of the instances of p, by node, unsetting
// those of the nodes not given.
func setAffine(t *testing.T, p naming.Path, avails map[string]status.T) {
	for _, n := range []string{"node1", "node2", "node3"} {
		if avail, ok := avails[n]; ok {
			instance.StatusData.Set(p, n, &instance.Status{Avail: avail})
		} else {
			instance.StatusData.Unset(p, n)
		}
	}
	t.Cleanup(func() {
		for _, n := range []string{"node1", "node2", "node3"} {
			instance.StatusData.Unset(p, n)
		}
	})
}

// An object starts only where the objects its hard affinity names are up, and
// where those its hard anti affinity names are not, whoever asks: the start
// command refuses the others too.
func TestHardAffinityExcludesStartCandidates(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-hard"}
	m := newAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})

	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Up, "node3": status.Down})
	assert.False(t, m.isStartCandidateLeader(true), "node1 is no candidate, the affine object being down there")
	assert.Equal(t, "node2", m.nextPlacedAtCandidate())
	assert.False(t, m.isHardAffinitySatisfied("node3"), "node3 is no switch destination, the affine object being down there")
	ok, reason := m.isRequestedStartable()
	assert.True(t, ok, reason)

	setAffine(t, affine, map[string]status.T{"node1": status.Up})
	assert.True(t, m.isStartCandidateLeader(true))
	assert.True(t, m.isStartCandidateLeader(false))

	setAffine(t, affine, map[string]status.T{})
	assert.False(t, m.isStartCandidateLeader(false))
	assert.Equal(t, "", m.nextPlacedAtCandidate())
	ok, reason = m.isRequestedStartable()
	assert.False(t, ok, "a start no node can do is refused")
	assert.Contains(t, reason, "hard affinity with affine-hard not satisfied on node1")
}

func TestHardAntiAffinityExcludesStartCandidates(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-hard-anti"}
	m := newAffinityTestManager(instance.ActorConfig{HardAntiAffinity: naming.Paths{affine}})

	setAffine(t, affine, map[string]status.T{"node1": status.Up, "node2": status.StandbyUp})
	assert.False(t, m.isStartCandidateLeader(true), "node1 is no candidate, the anti affine object being up there")
	assert.Equal(t, "node2", m.nextPlacedAtCandidate(), "a standby up anti affine instance excludes nothing")

	setAffine(t, affine, map[string]status.T{"node2": status.Up})
	assert.True(t, m.isStartCandidateLeader(true), "a node with no anti affine instance is a candidate")
}

// The soft affinities rank the candidates ahead of the placement policy,
// which orders those they rank the same.
func TestSoftAffinityRanksStartCandidates(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-soft"}
	anti := naming.Path{Kind: naming.KindSvc, Name: "affine-soft-anti"}
	m := newAffinityTestManager(instance.ActorConfig{
		SoftAffinity:     naming.Paths{affine},
		SoftAntiAffinity: naming.Paths{anti},
	})

	setAffine(t, affine, map[string]status.T{"node3": status.Up})
	setAffine(t, anti, map[string]status.T{"node1": status.Up})
	assert.Equal(t, []string{"node3", "node2", "node1"}, m.sortBySoftAffinity([]string{"node1", "node2", "node3"}))
	assert.False(t, m.isStartCandidateLeader(true))
	assert.Equal(t, "node3", m.nextPlacedAtCandidate())

	setAffine(t, affine, map[string]status.T{})
	setAffine(t, anti, map[string]status.T{})
	assert.True(t, m.isStartCandidateLeader(true), "with no preference, the placement policy decides")
}

// setAffineConfig publishes the configuration of p on the nodes, with its
// hard affinities and priority.
func setAffineConfig(t *testing.T, p naming.Path, prio priority.T, hardAffinity ...naming.Path) {
	for _, n := range []string{"node1", "node2", "node3"} {
		instance.ConfigData.Set(p, n, &instance.Config{
			Path:        p,
			Priority:    prio,
			ActorConfig: &instance.ActorConfig{HardAffinity: hardAffinity},
		})
	}
	t.Cleanup(func() {
		for _, n := range []string{"node1", "node2", "node3"} {
			instance.ConfigData.Unset(p, n)
		}
	})
}

// Objects whose hard affinities name each other, none up anywhere, would
// wait for each other forever: the one of the smallest priority, then of
// the first path, starts on its own, and the others follow it.
func TestHardAffinityCycleStartsOneFirst(t *testing.T) {
	a := naming.Path{Kind: naming.KindSvc, Name: "cycle-a"}
	b := naming.Path{Kind: naming.KindSvc, Name: "cycle-b"}
	newCycleManager := func(p, other naming.Path, prio priority.T) *Manager {
		m := newAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{other}})
		m.path = p
		m.instConfig.Path = p
		m.instConfig.Priority = prio
		return m
	}
	setAffine(t, a, map[string]status.T{"node1": status.Down, "node2": status.Down, "node3": status.Down})
	setAffine(t, b, map[string]status.T{"node1": status.Down, "node2": status.Down, "node3": status.Down})

	setAffineConfig(t, a, priority.Default, b)
	setAffineConfig(t, b, priority.Default, a)
	ma := newCycleManager(a, b, priority.Default)
	mb := newCycleManager(b, a, priority.Default)
	assert.Equal(t, naming.Paths{a, b}, ma.hardAffinityCycle(b), "same priority: the first path starts first")
	assert.True(t, ma.isStartCandidateLeader(true), "cycle-a starts on its placement leader")
	assert.False(t, mb.isStartCandidateLeader(true), "cycle-b follows cycle-a")
	assert.Contains(t, strings.Join(mb.hardAffinityViolations("node1"), ","), "cycle-a starting first")
	ok, reason := mb.isRequestedStartable()
	assert.True(t, ok, "a start of cycle-b follows cycle-a, started with it or later: %s", reason)

	setAffineConfig(t, b, 10, a)
	mb = newCycleManager(b, a, 10)
	assert.True(t, mb.isStartCandidateLeader(true), "the smallest priority starts first")
	assert.False(t, ma.isStartCandidateLeader(true))

	setAffine(t, b, map[string]status.T{"node1": status.Down, "node2": status.Up, "node3": status.Down})
	assert.Nil(t, ma.hardAffinityCycle(b), "a member up is followed")
	assert.False(t, ma.isStartCandidateLeader(true), "cycle-a follows cycle-b to node2")
	assert.Equal(t, "node2", ma.nextPlacedAtCandidate())
}

// An object naming another in its hard affinity, which names it in none, is
// no cycle: it waits for the other.
func TestHardAffinityWithoutCycleWaits(t *testing.T) {
	a := naming.Path{Kind: naming.KindSvc, Name: "nocycle-a"}
	b := naming.Path{Kind: naming.KindSvc, Name: "nocycle-b"}
	setAffine(t, b, map[string]status.T{"node1": status.Down})
	setAffineConfig(t, b, priority.Default)
	m := newAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{b}})
	m.path = a
	assert.Nil(t, m.hardAffinityCycle(b))
	assert.False(t, m.isStartCandidateLeader(true))
}
