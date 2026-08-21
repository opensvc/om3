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
	"github.com/google/uuid"
)

// subFilterKey uniquely identifies a filter within a subscription.
// Used as the key in the label index to track which specific filter
// contains each label, allowing efficient verification of only the
// relevant filters during matching.
type subFilterKey struct {
	subID    uuid.UUID
	filterIdx int
}

// invertedIndex implements efficient label-based subscription matching.
// It maps label key-value pairs to the set of (subscription, filter) pairs that have that label.
//
// Note: This index is designed to be used in a single-threaded context (e.g.,
// serialized by the bus command channel). No mutex is needed for the intended
// usage pattern where all operations are sequential.
type invertedIndex struct {
	// labelIndex: flatKey -> subFilterKey -> struct{}
	// Maps each label key:value pair to the set of (subscription, filter) pairs that have it.
	// flatKey format: "key:value" for regular labels, "__datatype__:value" for dataType.
	labelIndex map[string]map[subFilterKey]struct{}

	// subFilters: subscriptionID -> []filter
	// Stores the complete set of filters for each subscription
	subFilters map[uuid.UUID][]filter

	// subMinLabelCount: subscriptionID -> int
	// Minimum number of labels in any filter for each subscription.
	// Used for early pruning: if min > len(msgLabels), no filter can match.
	subMinLabelCount map[uuid.UUID]int

	// matchAllSubs: subscriptionID -> filterIdx
	// Tracks filters with empty dataType and empty labels (match everything)
	// These are not indexed by labels, so need special handling
	matchAllSubs map[uuid.UUID][]int
}

// newInvertedIndex creates a new inverted index.
func newInvertedIndex() *invertedIndex {
	return &invertedIndex{
		labelIndex:        make(map[string]map[subFilterKey]struct{}),
		subFilters:        make(map[uuid.UUID][]filter),
		subMinLabelCount: make(map[uuid.UUID]int),
		matchAllSubs:      make(map[uuid.UUID][]int),
	}
}

// AddSubscription adds a subscription with the given filters to the index.
// Must be called from a single thread (serialized by the bus command channel).
func (idx *invertedIndex) AddSubscription(subID uuid.UUID, filters []filter) {
	// Remove any existing entry for this subscription
	idx.removeSubscription(subID)

	// Track minimum number of labels across all filters for this subscription
	// Also track match-all filters (empty dataType and empty labels)
	minLabelCount := -1
	var matchAllFilterIdxs []int
	for filterIdx, f := range filters {
		// Index each label in this filter with the filter index
		sfk := subFilterKey{subID: subID, filterIdx: filterIdx}
		for key, value := range f.labels {
			flatKey := key + ":" + value
			if _, ok := idx.labelIndex[flatKey]; !ok {
				idx.labelIndex[flatKey] = make(map[subFilterKey]struct{})
			}
			idx.labelIndex[flatKey][sfk] = struct{}{}
		}
		// Index dataType as a special label for non-empty dataTypes
		// This allows using dataType as a filter dimension during lookup
		if f.dataType != "" {
			flatKey := "__datatype__:" + f.dataType
			if _, ok := idx.labelIndex[flatKey]; !ok {
				idx.labelIndex[flatKey] = make(map[subFilterKey]struct{})
			}
			idx.labelIndex[flatKey][sfk] = struct{}{}
		} else if len(f.labels) == 0 {
			// Match-all filter: empty dataType and empty labels
			matchAllFilterIdxs = append(matchAllFilterIdxs, filterIdx)
		}
		// Track minimum label count across all filters
		labelCount := len(f.labels)
		if minLabelCount < 0 || labelCount < minLabelCount {
			minLabelCount = labelCount
		}
	}
	// If all filters have no labels, minLabelCount remains -1, set to 0
	if minLabelCount < 0 {
		minLabelCount = 0
	}

	idx.subFilters[subID] = filters
	idx.subMinLabelCount[subID] = minLabelCount
	idx.matchAllSubs[subID] = matchAllFilterIdxs
}

// RemoveSubscription removes a subscription from the index.
// Must be called from a single thread (serialized by the bus command channel).
func (idx *invertedIndex) RemoveSubscription(subID uuid.UUID) {
	idx.removeSubscription(subID)
}

// removeSubscription removes a subscription from the index.
func (idx *invertedIndex) removeSubscription(subID uuid.UUID) {
	delete(idx.subFilters, subID)
	delete(idx.subMinLabelCount, subID)
	delete(idx.matchAllSubs, subID)

	// Remove from all label index entries (including __datatype__)
	for flatKey, sfkMap := range idx.labelIndex {
		// Delete all entries for this subscription
		for sfk := range sfkMap {
			if sfk.subID == subID {
				delete(sfkMap, sfk)
			}
		}
		if len(sfkMap) == 0 {
			delete(idx.labelIndex, flatKey)
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
//
// Must be called from a single thread (serialized by the bus command channel).
func (idx *invertedIndex) FindMatchingSubscriptions(dataType string, msgLabels Labels) []uuid.UUID {
	if len(idx.subFilters) == 0 {
		return nil
	}

	// Collect candidate (subscription, filter) pairs: those that have at least one label
	// matching the message (including dataType as a synthetic label). This allows us to
	// verify only the relevant filters.
	candidateFilterKeys := make(map[subFilterKey]struct{})

	// Build effective flat keys: include dataType as a special label if present
	// For regular labels: "key:value"
	// For dataType: "__datatype__:value"
	effectiveFlatKeys := make(map[string]struct{})
	for k, v := range msgLabels {
		effectiveFlatKeys[k+":"+v] = struct{}{}
	}
	if dataType != "" {
		effectiveFlatKeys["__datatype__:"+dataType] = struct{}{}
	}

	// If there are effective flat keys, find (sub, filter) pairs with matching labels
	if len(effectiveFlatKeys) > 0 {
		for flatKey := range effectiveFlatKeys {
			if sfkMap, ok := idx.labelIndex[flatKey]; ok {
				for sfk := range sfkMap {
					candidateFilterKeys[sfk] = struct{}{}
				}
			}
		}
		// Also include match-all filters (empty dataType and empty labels)
		// These match everything, so need to be candidates for any message
		for subID, filterIdxs := range idx.matchAllSubs {
			for _, filterIdx := range filterIdxs {
				sfk := subFilterKey{subID: subID, filterIdx: filterIdx}
				candidateFilterKeys[sfk] = struct{}{}
			}
		}
	} else {
		// Message has no labels and no dataType - all filters are candidates
		for subID, filters := range idx.subFilters {
			for filterIdx := range filters {
				sfk := subFilterKey{subID: subID, filterIdx: filterIdx}
				candidateFilterKeys[sfk] = struct{}{}
			}
		}
		// Also include match-all filters
		for subID, filterIdxs := range idx.matchAllSubs {
			for _, filterIdx := range filterIdxs {
				sfk := subFilterKey{subID: subID, filterIdx: filterIdx}
				candidateFilterKeys[sfk] = struct{}{}
			}
		}
	}

	// Check each candidate filter, deduplicating by subscription ID
	result := make([]uuid.UUID, 0, len(candidateFilterKeys))
	seenSubs := make(map[uuid.UUID]struct{})
	for sfk := range candidateFilterKeys {
		// Skip if we've already found a match for this subscription
		if _, ok := seenSubs[sfk.subID]; ok {
			continue
		}

		// Early pruning: if the minimum number of labels in any filter for this
		// subscription is greater than the number of message labels, no filter
		// can match (a filter with N labels cannot match a message with <N labels)
		if minLabels := idx.subMinLabelCount[sfk.subID]; minLabels > len(msgLabels) {
			continue
		}

		// Verify only this specific filter
		filters := idx.subFilters[sfk.subID]
		if sfk.filterIdx >= len(filters) {
			continue
		}
		f := filters[sfk.filterIdx]

		// Check dataType match
		if f.dataType != "" && f.dataType != dataType {
			continue
		}

		// Check if all filter labels are in message labels
		if len(f.labels) == 0 {
			// No label filters, dataType already matched - matches everything
			result = append(result, sfk.subID)
			seenSubs[sfk.subID] = struct{}{}
			continue
		}

		// If message has no labels but filter has labels, can't match
		if len(msgLabels) == 0 {
			continue
		}

		// Verify all labels in this filter match the message
		allMatch := true
		for key, value := range f.labels {
			if msgLabels[key] != value {
				allMatch = false
				break
			}
		}
		if allMatch {
			result = append(result, sfk.subID)
			seenSubs[sfk.subID] = struct{}{}
		}
	}

	return result
}
