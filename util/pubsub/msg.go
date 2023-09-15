package pubsub

import "github.com/google/uuid"

type (
	// SubscriptionError is an emitted publication made when a subscriber notification
	// exceeds its timeout
	SubscriptionError struct {
		Msg
		Id   uuid.UUID `json:"id"`
		Name string    `json:"name"`
		ErrS string    `json:"error"`
	}

	// SubscriptionQueueThreshold is an emitted publication made when a subscriber queue
	// reach/leave its current high threshold value
	SubscriptionQueueThreshold struct {
		Msg
		Id   uuid.UUID
		Name string `json:"name"`

		// Count is the current used slots in internal subscriber queue
		Count uint64 `json:"count"`

		// From is the previous high threshold value
		From uint64 `json:"from"`

		// To is the new high threshold value
		To uint64 `json:"to"`

		// Limit is the maximum queue size
		Limit uint64 `json:"limit"`
	}
)

func (m SubscriptionError) Kind() string {
	return "SubscriptionError"
}

func (m SubscriptionQueueThreshold) Kind() string {
	return "SubscriptionQueueThreshold"
}
