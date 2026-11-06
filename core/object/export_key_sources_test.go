package object

import (
	"context"
	"fmt"
	"time"
)

// SeedKeySources is seedKeySources, for the tests of the package
// object_test, which load the drivers the keywords seeding keys are of.
func SeedKeySources(ctx context.Context, o any, provision bool) error {
	i, ok := o.(interface {
		seedKeySources(context.Context, bool) error
	})
	if !ok {
		return fmt.Errorf("%T seeds no key", o)
	}
	return i.seedKeySources(ctx, provision)
}

// SetRandomKeyWait sets how long the provision of a node other than the
// leader waits for the keys the leader generates, and returns the function
// restoring the former values.
func SetRandomKeyWait(timeout, interval time.Duration) func() {
	formerTimeout, formerInterval := randomKeyWaitTimeout, randomKeyWaitInterval
	randomKeyWaitTimeout, randomKeyWaitInterval = timeout, interval
	return func() {
		randomKeyWaitTimeout, randomKeyWaitInterval = formerTimeout, formerInterval
	}
}
