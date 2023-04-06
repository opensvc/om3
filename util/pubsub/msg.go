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
)

func (m SubscriptionError) Kind() string {
	return "SubscriptionError"
}
