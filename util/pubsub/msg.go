package pubsub

import "github.com/google/uuid"

type (
	// SubscriptionError is an emitted publication made when a subscriber notification
	// exceeds its timeout
	SubscriptionError struct {
		Msg
		Id    uuid.UUID
		Name  string
		Error error
	}

	// SubscriptionQueueThreshold is an emitted publication made when a subscriber queue
	// reach/leave its current max value
	SubscriptionQueueThreshold struct {
		Msg
		Id    uuid.UUID
		Name  string
		Value uint64
		Next  uint64
	}
)

func (m SubscriptionError) Kind() string {
	return "SubscriptionError"
}

func (m SubscriptionQueueThreshold) Kind() string {
	return "SubscriptionQueueThreshold"
}
