package object

import (
	"fmt"
	"strings"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/render/tree"
)

// Render returns a human friendly string representation of the type instance.
func (t InstanceStates) Render() string {
	tree := t.Tree()
	return tree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t InstanceStates) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

//
// LoadTreeNode add the tree nodes representing the type instance into another
// tree, at the specified node.
//
func (t InstanceStates) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Node.Name).SetColor(config.Node.Color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(config.ColoredStatus(t.Status.Avail))
	head.AddColumn().AddText(t.descString())

	lastSubset := ""
	subsetNode := head
	for _, r := range t.Status.SortedResources() {
		if lastSubset != r.Subset {
			if r.Subset == "" {
				subsetNode = head
			} else {
				resourceSetName := r.ResourceID.DriverGroup().Name() + ":" + r.Subset
				subsetNode = head.AddNode()
				subsetNode.AddColumn().AddText(resourceSetName)
				subsetNode.AddColumn()
				subsetNode.AddColumn()
				parallel := ""
				if subset, ok := t.Status.Subsets[resourceSetName]; ok {
					if subset.Parallel {
						parallel = "//"
					}
				}
				subsetNode.AddColumn().AddText(parallel)
			}
			lastSubset = r.Subset
		}
		n := subsetNode.AddNode()
		n.AddColumn().AddText(r.ResourceID.Name)
		n.AddColumn().AddText(t.Status.resourceFlagsString(r.ResourceID, r))
		n.AddColumn().AddText(config.ColoredStatus(r.Status))
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

func (t InstanceStates) descString() string {
	l := make([]string, 0)

	// Overall
	if t.Status.Overall == status.Warn {
		l = append(l, config.ColoredStatus(t.Status.Overall))
	}

	// Frozen
	if !t.Status.Frozen.IsZero() {
		l = append(l, config.Node.Colorize.Frozen("frozen"))
	}

	// Node frozen
	if !t.Node.Frozen.IsZero() {
		l = append(l, config.Node.Colorize.Frozen("node-frozen"))
	}

	// Constraints
	if t.Status.Constraints {
		l = append(l, config.Node.Colorize.Error("constraints-violation"))
	}

	// Provisioned
	switch t.Status.Provisioned {
	case provisioned.False:
		l = append(l, config.Node.Colorize.Error("not-provisioned"))
	case provisioned.Mixed:
		l = append(l, config.Node.Colorize.Error("mix-provisioned"))
	}

	// Priority
	if s := t.Status.Priority.StatusString(); s != "" {
		l = append(l, config.Node.Colorize.Secondary(s))
	}

	// Monitor status
	switch t.Status.Monitor.Status {
	case "":
		l = append(l, config.Node.Colorize.Secondary("idle"))
	case "idle":
		l = append(l, config.Node.Colorize.Secondary(t.Status.Monitor.Status))
	default:
		l = append(l, config.Node.Colorize.Primary(t.Status.Monitor.Status))
	}

	// Monitor global expect
	if s := t.Status.Monitor.GlobalExpect; s != "" {
		l = append(l, s)
	}

	// Daemon down
	if t.Status.Monitor.StatusUpdated.IsZero() {
		l = append(l, config.Node.Colorize.Warning("daemon-down"))
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
func (t InstanceStatus) resourceFlagsString(rid ResourceID, r ResourceStatus) string {
	flags := ""

	// Running task or sync
	if t.Running.Has(rid.Name) {
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
	retries, _ = t.Monitor.Restart[rid.Name]
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
