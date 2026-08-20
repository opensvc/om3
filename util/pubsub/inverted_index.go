// Package pubsub - Inverted Index for label-based subscription matching
//
// This replaces the power-set combinations approach with an efficient inverted index.
// Instead of generating 2^n combinations for n labels, we store subscriptions
// indexed by individual label key-value pairs and perform set intersection at
// publish time.
//
// The inverted index implements subset matching: a message with labels M matches
// a subscription with filter labels F if M is a superset of F (M ⊇ F).
package pubsub

import (
	"sync"

	"github.com/google/uuid"
)

// invertedIndex implements efficient label-based subscription matching.
// It maps label key-value pairs to the set of subscriptions that have that label.
type invertedIndex struct {
	mu sync.RWMutex

	// labelIndex: key -> value -> subscriptionID -> struct{}
	// Maps each label key-value pair to the set of subscriptions that have it
	labelIndex map[string]map[string]map[uuid.UUID]struct{}

	// subFilters: subscriptionID -> []filter
	// Stores the complete set of filters for each subscription
	subFilters map[uuid.UUID][]filter

	// subLabelCount: subscriptionID -> int
	// Number of label filters for each subscription (for quick superset check)
	subLabelCount map[uuid.UUID]int
}

// newInvertedIndex creates a new inverted index.
func newInvertedIndex() *invertedIndex {
	return &invertedIndex{
		labelIndex:    make(map[string]map[string]map[uuid.UUID]struct{}),
		subFilters:    make(map[uuid.UUID][]filter),
		subLabelCount: make(map[uuid.UUID]int),
	}
}

// AddSubscription adds a subscription with the given filters to the index.
func (idx *invertedIndex) AddSubscription(subID uuid.UUID, filters []filter) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove any existing entry for this subscription
	idx.removeSubscriptionLockHeld(subID)

	labelCount := 0
	for _, f := range filters {
		// Count non-empty label filters (dataType is handled separately)
		if len(f.labels) > 0 {
			labelCount++
			// Index each label in this filter
			for key, value := range f.labels {
				if _, ok := idx.labelIndex[key]; !ok {
					idx.labelIndex[key] = make(map[string]map[uuid.UUID]struct{})
				}
				if _, ok := idx.labelIndex[key][value]; !ok {
					idx.labelIndex[key][value] = make(map[uuid.UUID]struct{})
				}
				idx.labelIndex[key][value][subID] = struct{}{}
			}
		}
	}

	idx.subFilters[subID] = filters
	idx.subLabelCount[subID] = labelCount
}

// RemoveSubscription removes a subscription from the index.
func (idx *invertedIndex) RemoveSubscription(subID uuid.UUID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeSubscriptionLockHeld(subID)
}

// removeSubscriptionLockHeld removes a subscription. Must be called with mu held.
func (idx *invertedIndex) removeSubscriptionLockHeld(subID uuid.UUID) {
	delete(idx.subFilters, subID)
	delete(idx.subLabelCount, subID)

	// Remove from all label index entries
	for key, valueMap := range idx.labelIndex {
		for value, subMap := range valueMap {
			delete(subMap, subID)
			if len(subMap) == 0 {
				delete(valueMap, value)
			}
		}
		if len(valueMap) == 0 {
			delete(idx.labelIndex, key)
		}
	}
}

// FindMatchingSubscriptions returns all subscription IDs that match the given
// message labels and data type.
//
// A subscription matches if at least one of its filters matches, where a filter matches if:
// 1. The filter's dataType is empty or matches the message dataType
// 2. All of the filter's labels are present in the message labels with matching values
//    (i.e., filter_labels ⊆ message_labels)
func (idx *invertedIndex) FindMatchingSubscriptions(dataType string, msgLabels Labels) []uuid.UUID {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.subFilters) == 0 {
		return nil
	}

	// Collect candidate subscriptions: those that have at least one label
	// matching the message. We'll then verify full filter match.
	candidateSubs := make(map[uuid.UUID]struct{})

	// If message has labels, find subscriptions with matching labels
	if len(msgLabels) > 0 {
		for key, value := range msgLabels {
			if valueMap, ok := idx.labelIndex[key]; ok {
				if subMap, ok := valueMap[value]; ok {
					for subID := range subMap {
						candidateSubs[subID] = struct{}{}
					}
				}
			}
		}
		// Also include subscriptions that might have empty-label filters
		// (they won't be in labelIndex but could still match via dataType)
		for subID, filters := range idx.subFilters {
			for _, f := range filters {
				// If filter has no labels and matches dataType, include it
				if len(f.labels) == 0 && (f.dataType == "" || f.dataType == dataType) {
					candidateSubs[subID] = struct{}{}
					break
				}
			}
		}
	} else {
		// Message has no labels - all subscriptions are candidates
		// (we'll filter by dataType and empty label filters below)
		for subID := range idx.subFilters {
			candidateSubs[subID] = struct{}{}
		}
	}

	// Check each candidate subscription
	var result []uuid.UUID
	for subID := range candidateSubs {
		if idx.subscriptionMatches(subID, dataType, msgLabels) {
			result = append(result, subID)
		}
	}

	return result
}

// subscriptionMatches checks if a subscription matches the message.
func (idx *invertedIndex) subscriptionMatches(subID uuid.UUID, dataType string, msgLabels Labels) bool {
	filters, ok := idx.subFilters[subID]
	if !ok {
		return false
	}

	for _, f := range filters {
		// Check dataType match
		if f.dataType != "" && f.dataType != dataType {
			continue
		}

		// Check if all filter labels are in message labels
		if len(f.labels) == 0 {
			// No label filters, dataType already matched - matches everything
			return true
		}

		// If message has no labels but filter has labels, can't match
		if len(msgLabels) == 0 {
			continue
		}

		allMatch := true
		for key, value := range f.labels {
			if msgLabels[key] != value {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true
		}
	}

	return false
}
