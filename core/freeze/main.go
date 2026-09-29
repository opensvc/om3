// Package freeze handles the frozen flags of the node and of the object
// instances, which keep the daemon from acting on its own initiative.
//
// A frozen flag records the scope of the freeze it was raised by. A node
// coming back after a freeze of the whole cluster or of a whole object, which
// it missed while it was down, adopts it from its peers. A freeze of a single
// node or instance, and a freeze the daemon took on its own, stay where they
// were raised.
package freeze

import (
	"bytes"
	"time"

	"github.com/opensvc/om3/v3/core/flagfile"
)

type (
	// Scope is what a freeze was asked for.
	Scope string
)

const (
	// ScopeCluster is a node frozen by a freeze of the cluster.
	ScopeCluster Scope = "cluster"

	// ScopeNode is a node frozen alone.
	ScopeNode Scope = "node"

	// ScopeObject is an instance frozen by a freeze of its object.
	ScopeObject Scope = "object"

	// ScopeInstance is an instance frozen alone.
	ScopeInstance Scope = "instance"
)

// Freeze raises the frozen flag at p, for this node or instance alone.
func Freeze(p string) error {
	return flagfile.Set(p)
}

// FreezeScope raises the frozen flag at p, for the freeze of the scope.
// A flag already raised keeps the scope of the freeze that raised it.
func FreezeScope(p string, scope Scope) error {
	return flagfile.SetWith(p, []byte(scope))
}

func Unfreeze(p string) error {
	return flagfile.Unset(p)
}

func Frozen(p string) time.Time {
	return flagfile.At(p)
}

// ScopeOf returns the scope of the freeze of the flag at p: wide when the
// flag was raised by the freeze of that scope, local otherwise, and empty
// when it is not raised.
func ScopeOf(p string, wide, local Scope) Scope {
	if flagfile.At(p).IsZero() {
		return ""
	}
	if string(bytes.TrimSpace(flagfile.Content(p))) == string(wide) {
		return wide
	}
	return local
}
