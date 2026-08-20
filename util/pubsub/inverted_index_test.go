package pubsub

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInvertedIndex_Basic(t *testing.T) {
	idx := newInvertedIndex()

	// Create test filters
	// Filter 1: matches messages with node=host1
	filters1 := []filter{
		{dataType: "TypeA", labels: Labels{"node": "host1"}},
	}
	// Filter 2: matches messages with node=host1 AND path=/svc
	filters2 := []filter{
		{dataType: "TypeA", labels: Labels{"node": "host1", "path": "/svc"}},
	}
	// Filter 3: matches messages with node=host2
	filters3 := []filter{
		{dataType: "TypeB", labels: Labels{"node": "host2"}},
	}

	subID1 := uuid.New()
	subID2 := uuid.New()
	subID3 := uuid.New()

	idx.AddSubscription(subID1, filters1)
	idx.AddSubscription(subID2, filters2)
	idx.AddSubscription(subID3, filters3)

	// Test: Message with node=host1 should match sub1 but NOT sub2
	msgLabels1 := Labels{"node": "host1"}
	result := idx.FindMatchingSubscriptions("TypeA", msgLabels1)
	require.Contains(t, result, subID1, "sub1 should match")
	require.NotContains(t, result, subID2, "sub2 should NOT match")
	require.NotContains(t, result, subID3)

	// Test: Message with node=host1, path=/svc should match both sub1 and sub2
	msgLabels2 := Labels{"node": "host1", "path": "/svc"}
	result = idx.FindMatchingSubscriptions("TypeA", msgLabels2)
	require.Contains(t, result, subID1)
	require.Contains(t, result, subID2)
	require.NotContains(t, result, subID3)

	// Test: Message with node=host2 should match sub3
	msgLabels3 := Labels{"node": "host2"}
	result = idx.FindMatchingSubscriptions("TypeB", msgLabels3)
	require.Contains(t, result, subID3)
	require.NotContains(t, result, subID1)
	require.NotContains(t, result, subID2)

	// Test: Wrong dataType shouldn't match
	result = idx.FindMatchingSubscriptions("TypeC", msgLabels1)
	require.Empty(t, result)

	// Test: Remove subscription
	idx.RemoveSubscription(subID1)
	result = idx.FindMatchingSubscriptions("TypeA", msgLabels1)
	require.NotContains(t, result, subID1)
	require.NotContains(t, result, subID2)
}

func TestInvertedIndex_NoLabels(t *testing.T) {
	idx := newInvertedIndex()

	// Subscription with no labels (matches all messages of its dataType)
	filters1 := []filter{
		{dataType: "TypeA", labels: Labels{}},
	}

	// Subscription with specific labels
	filters2 := []filter{
		{dataType: "TypeA", labels: Labels{"node": "host1"}},
	}

	subID1 := uuid.New()
	subID2 := uuid.New()

	idx.AddSubscription(subID1, filters1)
	idx.AddSubscription(subID2, filters2)

	// Message with no labels should match sub1 (empty filter)
	msgLabels := Labels{}
	result := idx.FindMatchingSubscriptions("TypeA", msgLabels)
	require.Contains(t, result, subID1, "sub1 should match: has empty filter for TypeA")
	require.NotContains(t, result, subID2, "sub2 should NOT match: requires node=host1")

	// Message with labels should match both (sub1 has no filter, sub2 matches)
	msgLabels = Labels{"node": "host1"}
	result = idx.FindMatchingSubscriptions("TypeA", msgLabels)
	require.Contains(t, result, subID1, "sub1 should match: has empty filter for TypeA")
	require.Contains(t, result, subID2, "sub2 should match: has filter {node=host1}")
}

func TestInvertedIndex_MultipleFiltersPerSub(t *testing.T) {
	idx := newInvertedIndex()

	// Subscription with multiple filters
	filters := []filter{
		{dataType: "TypeA", labels: Labels{"node": "host1"}},
		{dataType: "TypeA", labels: Labels{"node": "host2"}},
		{dataType: "TypeB", labels: Labels{"path": "/svc"}},
	}

	subID := uuid.New()
	idx.AddSubscription(subID, filters)

	// Should match via first filter
	result := idx.FindMatchingSubscriptions("TypeA", Labels{"node": "host1"})
	require.Contains(t, result, subID)

	// Should match via second filter
	result = idx.FindMatchingSubscriptions("TypeA", Labels{"node": "host2"})
	require.Contains(t, result, subID)

	// Should match via third filter
	result = idx.FindMatchingSubscriptions("TypeB", Labels{"path": "/svc"})
	require.Contains(t, result, subID)

	// Should NOT match - wrong node
	result = idx.FindMatchingSubscriptions("TypeA", Labels{"node": "host3"})
	require.NotContains(t, result, subID)
}

func TestInvertedIndex_SubsetMatching(t *testing.T) {
	idx := newInvertedIndex()

	// Subscription filter: {node=host1}
	filters := []filter{
		{dataType: "TypeA", labels: Labels{"node": "host1"}},
	}

	subID := uuid.New()
	idx.AddSubscription(subID, filters)

	// Message with only node=host1 should match
	result := idx.FindMatchingSubscriptions("TypeA", Labels{"node": "host1"})
	require.Contains(t, result, subID)

	// Message with node=host1 AND path=/svc should ALSO match
	// (message labels are superset of filter labels)
	result = idx.FindMatchingSubscriptions("TypeA", Labels{"node": "host1", "path": "/svc"})
	require.Contains(t, result, subID)

	// Message with node=host2 should NOT match
	result = idx.FindMatchingSubscriptions("TypeA", Labels{"node": "host2", "path": "/svc"})
	require.NotContains(t, result, subID)
}

