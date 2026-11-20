package object

import "github.com/opensvc/om3/v3/core/resource"

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
