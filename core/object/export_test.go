package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
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

// SetNetRuleObjects replaces who tells the validation the objects the
// rules of net_rules name, and returns the function restoring it.
func SetNetRuleObjects(fn func(objects []naming.Path, namespaces []string) (netrule.Objects, error)) func() {
	saved := netRuleObjects
	netRuleObjects = fn
	return func() { netRuleObjects = saved }
}

// SetNetRuleView replaces what tells the validation the networks and the
// nodes the rules of net_rules name, and returns the function restoring it.
func SetNetRuleView(fn func() (netrule.View, error)) func() {
	saved := netRuleView
	netRuleView = fn
	return func() { netRuleView = saved }
}

// ImageSourceClient is what the prefetch asks the daemon of the node, for
// the tests to answer in its place.
type ImageSourceClient = imageSourceClient

// SetImageSourceClient replaces the client the prefetch asks the daemon of
// the node with, and returns the function restoring it.
func SetImageSourceClient(c ImageSourceClient) func() {
	former := newImageSourceClient
	newImageSourceClient = func() (imageSourceClient, error) { return c, nil }
	return func() { newImageSourceClient = former }
}

// OrderImageSourceNodes exposes to the tests the order the nodes are asked
// for an image in.
func OrderImageSourceNodes(nodes []string, up map[string]bool) []string {
	return orderImageSourceNodes(nodes, up)
}
