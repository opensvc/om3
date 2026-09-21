// Package claimpool is a namespace's claim on the space of a pool.
//
// A namespace consumes things the cluster owns and its peers share. A claim
// says which resource, and how much of it the namespace may take, and each
// kind of resource is a driver of the claim group so a namespace says it the
// same way whatever it claims.
//
// The driver holds no behaviour. Counting what a namespace already holds of a
// pool, and refusing a claim that does not fit, is done in core/pool, beside
// the lookup that hands the pools out.
package claimpool

import (
	"github.com/opensvc/om3/v3/core/driver"
)

type (
	T struct{}
)

var (
	drvID = driver.NewID(driver.GroupClaim, "pool")
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
