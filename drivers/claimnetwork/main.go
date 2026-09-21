// Package claimnetwork is a namespace's claim on the addresses of a network.
//
// A namespace consumes things the cluster owns and its peers share. A claim
// says which resource, and how much of it the namespace may take, and each
// kind of resource is a driver of the claim group so a namespace says it the
// same way whatever it claims.
//
// The driver holds no behaviour. Counting the addresses a namespace already
// holds, and refusing an allocation that would take it past its claim, is done
// in core/network, beside the allocator itself.
package claimnetwork

import (
	"github.com/opensvc/om3/v3/core/driver"
)

type (
	T struct{}
)

var (
	drvID = driver.NewID(driver.GroupClaim, "network")
)

func init() {
	driver.Register(drvID, New)
}

func New() *T {
	return &T{}
}

func (t *T) DriverID() driver.ID {
	return drvID
}
