package dns

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// nameIndexFromScratch builds the name index the way a full rebuild would,
// to compare it with the incrementally maintained one.
func nameIndexFromScratch(t *Manager) map[string][]Record {
	index := make(map[string][]Record)
	for _, record := range t.clusterRecords {
		index[record.Name] = append(index[record.Name], record)
	}
	for _, recordMap := range t.state {
		for _, record := range recordMap {
			index[record.Name] = append(index[record.Name], record)
		}
	}
	return index
}

// requireIndexInSync verifies the name index holds, for each name, the same
// records as a full rebuild would. Both are compared as multisets: the index
// order is not significant, and a record indexed twice must stay indexed
// twice.
func requireIndexInSync(t *testing.T, m *Manager) {
	t.Helper()
	count := func(index map[string][]Record) map[Record]int {
		counted := make(map[Record]int)
		for name, records := range index {
			require.NotEmpty(t, records, "name %s is indexed with no record", name)
			for _, record := range records {
				require.Equal(t, name, record.Name, "record indexed under the wrong name")
				counted[record]++
			}
		}
		return counted
	}
	require.Equal(t, count(nameIndexFromScratch(m)), count(m.nameIndex))
}

func recordMapOf(records ...Record) map[recordKey]Record {
	recordMap := make(map[recordKey]Record)
	for _, record := range records {
		recordMap[record.Key()] = record
	}
	return recordMap
}

func TestNameIndexIsMaintainedIncrementally(t *testing.T) {
	m := &Manager{
		state:     make(map[stateKey]map[recordKey]Record),
		nameIndex: make(map[string][]Record),
		clusterConfig: cluster.Config{
			Name: "cluster1",
			DNS:  []string{"10.0.0.1", "10.0.0.2"},
		},
	}

	var (
		key1 = stateKey{path: "system/svc/svc1", node: "node1"}
		key2 = stateKey{path: "system/svc/svc1", node: "node2"}

		// the fqdn record both instances of svc1 yield, indexed twice
		shared = Record{Name: "svc1.system.svc.cluster1.", Type: "A", TTL: 60, Content: "10.1.1.1"}

		onNode1 = Record{Name: "svc1.system.svc.node1.cluster1.", Type: "A", TTL: 60, Content: "10.1.1.1"}
		onNode2 = Record{Name: "svc1.system.svc.node2.cluster1.", Type: "A", TTL: 60, Content: "10.1.1.2"}
	)

	m.setClusterRecords()
	requireIndexInSync(t, m)

	t.Run("index the records of a state key", func(t *testing.T) {
		m.setStateRecords(key1, recordMapOf(shared, onNode1))
		requireIndexInSync(t, m)
	})

	t.Run("index a record another state key already yields", func(t *testing.T) {
		m.setStateRecords(key2, recordMapOf(shared, onNode2))
		requireIndexInSync(t, m)
		require.Len(t, m.nameIndex[shared.Name], 2, "the shared record must be indexed once per state key")
	})

	t.Run("dropping a state key keeps the records of the others", func(t *testing.T) {
		m.setStateRecords(key1, nil)
		requireIndexInSync(t, m)
		require.NotContains(t, m.nameIndex, onNode1.Name)
		require.Len(t, m.nameIndex[shared.Name], 1, "the shared record must stay indexed for the remaining key")
	})

	t.Run("changing the records of a state key", func(t *testing.T) {
		changed := onNode2
		changed.Content = "10.1.1.3"
		m.setStateRecords(key2, recordMapOf(shared, changed))
		requireIndexInSync(t, m)
		require.Equal(t, []Record{changed}, m.nameIndex[changed.Name])
	})

	t.Run("changing the ttl of a record", func(t *testing.T) {
		reTTLed := shared
		reTTLed.TTL = 30
		m.setStateRecords(key2, recordMapOf(reTTLed))
		requireIndexInSync(t, m)
		require.Equal(t, []Record{reTTLed}, m.nameIndex[shared.Name])
	})

	t.Run("changing the cluster nameservers", func(t *testing.T) {
		m.clusterConfig.DNS = []string{"10.0.0.3"}
		m.setClusterRecords()
		requireIndexInSync(t, m)
		require.NotContains(t, m.nameIndex, "ns2.cluster1.")
	})

	t.Run("dropping the last state key empties the index", func(t *testing.T) {
		m.setStateRecords(key2, nil)
		requireIndexInSync(t, m)
		require.Empty(t, m.state)
		for name := range m.nameIndex {
			require.Contains(t, []string{"cluster1.", "ns1.cluster1."}, name, "only cluster records must be left")
		}
	})
}

type nopPublisher struct{}

func (nopPublisher) Pub(_ pubsub.Messager, _ ...pubsub.Label) {}

// dnsManager is a manager with just enough in it to stage the records of an
// instance status.
func dnsManager() *Manager {
	return &Manager{
		state:         make(map[stateKey]map[recordKey]Record),
		nameIndex:     make(map[string][]Record),
		publisher:     nopPublisher{},
		log:           plog.NewDefaultLogger(),
		clusterConfig: cluster.Config{Name: "cluster1"},
	}
}

// scopedStandbyIP is the shape that showed the resource status cannot decide:
// a failover object whose ipaddr is scoped, carried by a standby resource, so
// every node reports a different address and every one of them is "stdby up".
func scopedStandbyIP(avail status.T, addr string) *msgbus.InstanceStatusUpdated {
	return &msgbus.InstanceStatusUpdated{
		Path: naming.Path{Name: "svc11", Namespace: "root", Kind: naming.KindSvc},
		Value: instance.Status{
			Avail: avail,
			Resources: instance.ResourceStatuses{
				"ip#0": resource.Status{
					Status:    status.StandbyUp,
					IsStandby: true,
					Info:      map[string]any{ipAddrInfoKey: addr},
				},
			},
		},
	}
}

// aRecords is the recordset a name answers with, which for an object name is
// one address per serving instance.
func aRecords(m *Manager, name string) []string {
	contents := make([]string, 0)
	for _, record := range m.nameIndex[name] {
		if record.Type == "A" {
			contents = append(contents, record.Content)
		}
	}
	sort.Strings(contents)
	return contents
}

// The name of the object answers with the addresses of the instances serving
// it. A failover has one, whichever node it is on.
func TestOnlyAServingInstanceAnswersForTheObject(t *testing.T) {
	m := dnsManager()

	up := scopedStandbyIP(status.Up, "10.29.0.11")
	up.Node = "node1"
	m.onInstanceStatusUpdated(up)

	down := scopedStandbyIP(status.Down, "10.29.0.13")
	down.Node = "node3"
	m.onInstanceStatusUpdated(down)

	require.Equal(t, []string{"10.29.0.11"}, aRecords(m, "svc11.root.svc.cluster1."),
		"the object resolves to the address of the instance serving it, and to no other")
	require.Equal(t, []string{"10.29.0.11"}, aRecords(m, "svc11.root.svc.node1.node.cluster1."))
	require.Equal(t, []string{"10.29.0.13"}, aRecords(m, "svc11.root.svc.node3.node.cluster1."),
		"the node affine name answers for the instance on that node, serving or not")
	requireIndexInSync(t, m)
}

// A flex object is allowed several instances up at once, and the recordset of
// its name is all of their addresses.
func TestAFlexObjectAnswersWithEveryServingInstance(t *testing.T) {
	m := dnsManager()
	for node, addr := range map[string]string{"node1": "10.29.0.11", "node2": "10.29.0.12"} {
		c := scopedStandbyIP(status.Up, addr)
		c.Node = node
		m.onInstanceStatusUpdated(c)
	}
	require.Equal(t, []string{"10.29.0.11", "10.29.0.12"}, aRecords(m, "svc11.root.svc.cluster1."))
}

// A frozen object stays fully usable by its clients, so its address keeps
// resolving. Freezing does not change the availability, which is what this
// reads.
func TestAFrozenInstanceKeepsAnsweringForTheObject(t *testing.T) {
	m := dnsManager()
	c := scopedStandbyIP(status.Up, "10.29.0.11")
	c.Node = "node1"
	c.Value.FrozenAt = time.Now()
	m.onInstanceStatusUpdated(c)

	require.Equal(t, []string{"10.29.0.11"}, aRecords(m, "svc11.root.svc.cluster1."))
}

// An instance whose warning does not stop it serving still answers.
func TestAWarnInstanceAnswersForTheObject(t *testing.T) {
	m := dnsManager()
	c := scopedStandbyIP(status.Warn, "10.29.0.11")
	c.Node = "node1"
	m.onInstanceStatusUpdated(c)

	require.Equal(t, []string{"10.29.0.11"}, aRecords(m, "svc11.root.svc.cluster1."))
}

// An instance running one address and three containers keeps its address
// resolvable when one of the containers dies. The address is the ip
// resource's, and a failed database is no reason to make the object
// unreachable by name: the outage would read as a name resolution problem.
func TestADeadResourceDoesNotUnpublishTheAddress(t *testing.T) {
	for _, avail := range []status.T{status.Warn, status.Down} {
		t.Run(avail.String(), func(t *testing.T) {
			m := dnsManager()
			m.onInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{
				Path: naming.Path{Name: "pod1", Namespace: "root", Kind: naming.KindSvc},
				Node: "node1",
				Value: instance.Status{
					Avail: avail,
					Resources: instance.ResourceStatuses{
						"ip#0":        resource.Status{Status: status.Up, Info: map[string]any{ipAddrInfoKey: "10.100.0.240"}},
						"container#0": resource.Status{Status: status.Up},
						"container#1": resource.Status{Status: status.Up},
						"container#2": resource.Status{Status: status.Down},
					},
				},
			})
			require.Equal(t, []string{"10.100.0.240"}, aRecords(m, "pod1.root.svc.cluster1."),
				"the ip is up, so the object answers with it whatever died beside it")
		})
	}
}

// A resource that is down serves no address, which is what keeps the scoped
// address of a stopped instance out of the zone.
func TestADownResourceAnswersForNothing(t *testing.T) {
	m := dnsManager()
	m.onInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{
		Path: naming.Path{Name: "svc1", Namespace: "root", Kind: naming.KindSvc},
		Node: "node1",
		Value: instance.Status{
			Avail: status.Down,
			Resources: instance.ResourceStatuses{
				"ip#0": resource.Status{Status: status.Down, Info: map[string]any{ipAddrInfoKey: "128.0.0.10"}},
			},
		},
	})
	require.Empty(t, aRecords(m, "svc1.root.svc.cluster1."))
	require.Equal(t, []string{"128.0.0.10"}, aRecords(m, "svc1.root.svc.node1.node.cluster1."),
		"the node affine name still answers for the instance on that node")
}
