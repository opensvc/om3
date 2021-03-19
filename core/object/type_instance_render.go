package object

import (
	"fmt"
	"strings"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/render/tree"
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
	head.AddColumn().AddText(t.Nodename).SetColor(config.Node.Color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(t.Avail.ColorString())
	head.AddColumn().AddText(t.descString())

	for rid, r := range t.Resources {
		n := head.AddNode()
		n.AddColumn().AddText(rid)
		n.AddColumn().AddText(t.resourceFlagsString(rid, r)) // flags
		n.AddColumn().AddText(r.Status.ColorString())
		desc := n.AddColumn()
		desc.AddText(r.Label)
		for _, entry := range r.Log {
			t := desc.AddText(entry)
			switch {
			case strings.HasPrefix(entry, "error:"):
				t.SetColor(config.Node.Color.Error)
			case strings.HasPrefix(entry, "warn:"):
				t.SetColor(config.Node.Color.Warning)
			}
		}
	}
}

func (t InstanceStatus) descString() string {
	l := make([]string, 0)

	// Frozen
	if !t.Frozen.IsZero() {
		l = append(l, config.Node.Colorize.Frozen("frozen"))
	}

	// Priority
	if s := t.Priority.StatusString(); s != "" {
		l = append(l, config.Node.Colorize.Secondary(s))
	}

	// Overall
	l = append(l, t.Overall.ColorString())

	// Monitor status
	switch t.Monitor.Status {
	case "":
		l = append(l, config.Node.Colorize.Secondary("idle"))
	case "idle":
		l = append(l, config.Node.Colorize.Secondary(t.Monitor.Status))
	default:
		l = append(l, config.Node.Colorize.Primary(t.Monitor.Status))
	}

	// Monitor global expect
	if s := t.Monitor.GlobalExpect; s != "" {
		l = append(l, s)
	}

	return strings.Join(l, " ")
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
		flags += "."
	}

	flags += r.Monitor.FlagString()
	flags += r.Disable.FlagString()
	flags += r.Optional.FlagString()
	flags += r.Encap.FlagString()
	flags += r.Provisioned.State.FlagString()
	flags += r.Standby.FlagString()

	// Restart and retries
	retries := 0
	retries, _ = t.Monitor.Restart[rid]
	remaining := r.Restart - retries
	switch {
	case r.Restart <= 0:
		flags += "."
	case remaining < 0:
		flags += "0"
	case remaining < 10:
		flags += fmt.Sprintf("%d", remaining)
	default:
		flags += "+"
	}
	return flags
}
