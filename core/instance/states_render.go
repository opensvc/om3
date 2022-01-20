package instance

import (
	"strings"

	"opensvc.com/opensvc/core/colorstatus"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/render/tree"
)

// Render returns a human friendly string representation of the type instance.
func (t States) Render() string {
	newTree := t.Tree()
	return newTree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t States) Tree() *tree.Tree {
	newTree := tree.New()
	t.LoadTreeNode(newTree.Head())
	return newTree
}

//
// LoadTreeNode add the tree nodes representing the type instance into another
// tree, at the specified node.
//
func (t States) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Node.Name).SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(colorstatus.Sprint(t.Status.Avail, rawconfig.Node.Colorize))
	head.AddColumn().AddText(t.descString())

	lastSubset := ""
	subsetNode := head
	for _, r := range t.Status.SortedResources() {
		if lastSubset != r.Subset {
			if r.Subset == "" {
				subsetNode = head
			} else {
				resourceSetName := r.ResourceID.DriverGroup().String() + ":" + r.Subset
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
		n.AddColumn().AddText(t.Status.ResourceFlagsString(*r.ResourceID, r))
		n.AddColumn().AddText(colorstatus.Sprint(r.Status, rawconfig.Node.Colorize))
		desc := n.AddColumn()
		desc.AddText(r.Label)
		for _, entry := range r.Log {
			t := desc.AddText(entry.String())
			switch entry.Level {
			case "error":
				t.SetColor(rawconfig.Node.Color.Error)
			case "warn":
				t.SetColor(rawconfig.Node.Color.Warning)
			}
		}
	}
}

func (t States) descString() string {
	l := make([]string, 0)

	// Overall
	if t.Status.Overall == status.Warn {
		l = append(l, colorstatus.Sprint(t.Status.Overall, rawconfig.Node.Colorize))
	}

	// Frozen
	if !t.Status.Frozen.IsZero() {
		l = append(l, rawconfig.Node.Colorize.Frozen("frozen"))
	}

	// Node frozen
	if !t.Node.Frozen.IsZero() {
		l = append(l, rawconfig.Node.Colorize.Frozen("node-frozen"))
	}

	// Constraints
	if t.Status.Constraints {
		l = append(l, rawconfig.Node.Colorize.Error("constraints-violation"))
	}

	// Provisioned
	switch t.Status.Provisioned {
	case provisioned.False:
		l = append(l, rawconfig.Node.Colorize.Error("not-provisioned"))
	case provisioned.Mixed:
		l = append(l, rawconfig.Node.Colorize.Error("mix-provisioned"))
	}

	// Priority
	if s := t.Status.Priority.StatusString(); s != "" {
		l = append(l, rawconfig.Node.Colorize.Secondary(s))
	}

	// Monitor status
	switch t.Status.Monitor.Status {
	case "":
		l = append(l, rawconfig.Node.Colorize.Secondary("idle"))
	case "idle":
		l = append(l, rawconfig.Node.Colorize.Secondary(t.Status.Monitor.Status))
	default:
		l = append(l, rawconfig.Node.Colorize.Primary(t.Status.Monitor.Status))
	}

	// Monitor global expect
	if s := t.Status.Monitor.GlobalExpect; s != "" {
		l = append(l, s)
	}

	// Monitor local expect
	if s := t.Status.Monitor.LocalExpect; s != "" {
		l = append(l, s)
	}

	// Daemon down
	if t.Status.Monitor.StatusUpdated.IsZero() {
		l = append(l, rawconfig.Node.Colorize.Warning("daemon-down"))
	}

	return strings.Join(l, " ")
}
