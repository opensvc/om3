// Package claimcpu is a namespace's claim on the cpu its objects are capped to.
//
// A namespace consumes things the cluster owns and its peers share. A claim
// says which resource, and how much of it the namespace may take, and each
// kind of resource is a driver of the claim group so a namespace says it the
// same way whatever it claims.
//
// The driver holds no behaviour. What an object claims is computed from its
// caps in core/object, and the claim is weighed by the node speaking for the
// cluster when a configuration is written.
package claimcpu

import (
	"github.com/opensvc/om3/v3/core/driver"
)

type (
	T struct{}
)

var (
	drvID = driver.NewID(driver.GroupClaim, "cpu")
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
