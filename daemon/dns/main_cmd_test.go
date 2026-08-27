package dns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
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
