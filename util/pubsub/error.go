package pubsub

import (
	"github.com/google/uuid"
)

type (
	ErrSubscriptionIDNotFound struct {
		id uuid.UUID
	}
)

func (e ErrSubscriptionIDNotFound) Error() string {
	return "subscriber id " + e.id.String() + " not found"
}
