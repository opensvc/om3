package pubsub

import "github.com/google/uuid"

type (
	// SubscriptionError is an emitted publication made when a subscriber notification
	// exceeds its timeout
	SubscriptionError struct {
		Msg  `yaml:",inline"`
		Id   uuid.UUID `json:"id" yaml:"id"`
		Name string    `json:"name" yaml:"name"`
		ErrS string    `json:"error" yaml:"error"`
	}

	// SubscriptionQueueThreshold is an emitted publication made when a subscriber queue
	// reach/leave its current high threshold value
	SubscriptionQueueThreshold struct {
		Msg  `yaml:",inline"`
		Id   uuid.UUID
		Name string `json:"name" yaml:"name"`

		// Count is the current used slots in internal subscriber queue
		Count uint64 `json:"count" yaml:"count"`

		// From is the previous high threshold value
		From uint64 `json:"from" yaml:"from"`

		// To is the new high threshold value
		To uint64 `json:"to" yaml:"to"`

		// Limit is the maximum queue size
		Limit uint64 `json:"limit" yaml:"limit"`
	}
)

func (m SubscriptionError) Kind() string {
	return "SubscriptionError"
}

func (m SubscriptionQueueThreshold) Kind() string {
	return "SubscriptionQueueThreshold"
}
