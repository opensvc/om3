package object

import (
	"fmt"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/util/render/palette"
	"opensvc.com/opensvc/util/render/tree"
)

const (
	FlagEmpty = "."
)

// Render returns a human friendly string representation of the type instance.
func (t InstanceStatus) Render() string {
	tree := t.Tree()
	return tree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t InstanceStatus) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

//
// LoadTreeNode add the tree nodes representing the type instance into another
// tree, at the specified node.
//
func (t InstanceStatus) LoadTreeNode(head *tree.Node) {
	colors := palette.New(config.Node.Palette)
	head.AddColumn().AddText(t.Nodename).SetColor(colors.Bold)
	head.AddColumn()
	head.AddColumn().AddText(t.Avail.ColorString())
	head.AddColumn().AddText(fmt.Sprint(t.Priority)).SetColor(colors.Secondary)
	for rid, r := range t.Resources {
		n := head.AddNode()
		n.AddColumn().AddText(rid)
		n.AddColumn().AddText(t.resourceFlagsString(rid, r)) // flags
		n.AddColumn().AddText(r.Status.ColorString())
		n.AddColumn().AddText(r.Label)
	}
}

//
// resourceFlagsString formats resource flags as a vector of characters.
//
//   R  Running
//   M  Monitored
//   D  Disabled
//   O  Optional
//   E  Encap
//   P  Provisioned
//   S  Standby
//
func (t InstanceStatus) resourceFlagsString(rid string, r ResourceStatus) string {
	flags := ""

	// Running task or sync
	if t.Running.Has(rid) {
		flags += "R"
	} else {
		flags += FlagEmpty
	}

	// Monitored
	if r.Monitor {
		flags += "M"
	} else {
		flags += FlagEmpty
	}

	// Disabled
	if r.Disable {
		flags += "D"
	} else {
		flags += FlagEmpty
	}

	// Optional
	if r.Optional {
		flags += "O"
	} else {
		flags += FlagEmpty
	}

	// Encapsulated
	if r.Encap {
		flags += "E"
	} else {
		flags += FlagEmpty
	}

	// Provisioned
	switch r.Provisioned.State {
	case provisioned.True:
		flags += "."
	case provisioned.False:
		flags += "P"
	default:
		flags += "/"
	}

	// Standby
	if r.Standby {
		flags += "S"
	} else {
		flags += FlagEmpty
	}

	// Restart and retries
	retries := 0
	retries, _ = t.Monitor.Restart[rid]
	remaining_restart := r.Restart - retries
	switch {
	case r.Restart <= 0:
		flags += "."
	case remaining_restart < 0:
		flags += "0"
	case remaining_restart < 10:
		flags += fmt.Sprintf("%d", remaining_restart)
	default:
		flags += "+"
	}
	return flags
}
