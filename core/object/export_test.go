package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/resource"
)

// ResolveProvisionKeywords exposes to the tests the resolution a provision
// runs of the keywords read at provision only, on the resource r of the
// object o.
func ResolveProvisionKeywords(o any, r resource.Driver) {
	switch a := o.(type) {
	case *svc:
		a.resolveProvisionKeywords(r)
	case *vol:
		a.resolveProvisionKeywords(r)
	default:
		panic("ResolveProvisionKeywords: not an actor")
	}
}

// ResizeShortfall exposes to the tests what the status of the resource r of
// the object o says of a resize that has not finished.
func ResizeShortfall(o any, r resource.Driver) string {
	switch a := o.(type) {
	case *svc:
		return a.resizeShortfall(context.Background(), r)
	case *vol:
		return a.resizeShortfall(context.Background(), r)
	default:
		panic("ResizeShortfall: not an actor")
	}
}
