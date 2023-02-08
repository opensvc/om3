package instance

import (
	"strings"

	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/render/tree"
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

// LoadTreeNode add the tree nodes representing the type instance into another
// tree, at the specified node.
func (t States) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Node.Name).SetColor(rawconfig.Color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(colorstatus.Sprint(t.Status.Avail, rawconfig.Colorize))
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
		flags := t.Status.ResourceFlagsString(*r.ResourceID, r) + t.Monitor.ResourceFlagRestartString(*r.ResourceID, r)
		n := subsetNode.AddNode()
		n.AddColumn().AddText(r.ResourceID.Name)
		n.AddColumn().AddText(flags)
		n.AddColumn().AddText(colorstatus.Sprint(r.Status, rawconfig.Colorize))
		desc := n.AddColumn()
		desc.AddText(r.Label)
		for _, entry := range r.Log {
			t := desc.AddText(entry.String())
			switch entry.Level {
			case "error":
				t.SetColor(rawconfig.Color.Error)
			case "warn":
				t.SetColor(rawconfig.Color.Warning)
			}
		}
	}
}

func (t States) descString() string {
	l := make([]string, 0)

	// Overall
	if t.Status.Overall == status.Warn {
		l = append(l, colorstatus.Sprint(t.Status.Overall, rawconfig.Colorize))
	}

	// Frozen
	if !t.Status.Frozen.IsZero() {
		l = append(l, rawconfig.Colorize.Frozen("frozen"))
	}

	// Node frozen
	if !t.Node.Frozen.IsZero() {
		l = append(l, rawconfig.Colorize.Frozen("node-frozen"))
	}

	// Constraints
	if t.Status.Constraints {
		l = append(l, rawconfig.Colorize.Error("constraints-violation"))
	}

	// Provisioned
	switch t.Status.Provisioned {
	case provisioned.False:
		l = append(l, rawconfig.Colorize.Error("not-provisioned"))
	case provisioned.Mixed:
		l = append(l, rawconfig.Colorize.Error("mix-provisioned"))
	}

	// Priority
	if s := t.Config.Priority.StatusString(); s != "" {
		l = append(l, rawconfig.Colorize.Secondary(s))
	}

	// Monitor state
	switch t.Monitor.State {
	case MonitorStateIdle:
		l = append(l, rawconfig.Colorize.Secondary(t.Monitor.State.String()))
	default:
		l = append(l, rawconfig.Colorize.Primary(t.Monitor.State.String()))
	}

	// Monitor global expect
	if t.Monitor.GlobalExpect != MonitorGlobalExpectUnset {
		l = append(l, t.Monitor.GlobalExpect.String())
	}

	// Monitor local expect
	if t.Monitor.LocalExpect != MonitorLocalExpectUnset {
		l = append(l, t.Monitor.LocalExpect.String())
	}

	// Daemon down
	if t.Monitor.StateUpdated.IsZero() {
		l = append(l, rawconfig.Colorize.Warning("daemon-down"))
	}

	return strings.Join(l, " ")
}
