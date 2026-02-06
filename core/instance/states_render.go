package instance

import (
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/colorstatus"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceset"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/render/tree"
	"github.com/opensvc/om3/v3/util/xmap"
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

// LoadTreeNodeFolded add the tree nodes representing the type instance into another
// tree, at the specified node.
// TODO: probable bug, LoadTreeNodeFolded duplicate code of LoadTreeNode
func (t States) LoadTreeNodeFolded(head *tree.Node) {
	head.AddColumn().AddText(t.Node.Name).SetColor(rawconfig.Color.Bold)
	head.AddColumn()
	if t.Config.ActorConfig != nil {
		head.AddColumn().AddText(colorstatus.Sprint(t.Status.Avail, rawconfig.Colorize))
	} else {
		head.AddColumn()
	}
	head.AddColumn().AddText(t.descString())
}

// LoadTreeNode add the tree nodes representing the type instance into another
// tree, at the specified node.
func (t States) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Node.Name).SetColor(rawconfig.Color.Bold)
	head.AddColumn()
	switch t.Path.Kind {
	case naming.KindSec, naming.KindCfg, naming.KindCcfg, naming.KindUsr, naming.KindNscfg:
		head.AddColumn()
		head.AddColumn().AddText(t.descString())
		return
	}
	head.AddColumn().AddText(colorstatus.Sprint(t.Status.Avail, rawconfig.Colorize))
	head.AddColumn().AddText(t.descString())

	resNode := head.AddNode()
	resNode.AddColumn().AddText("resources")
	resNode.AddColumn()
	resNode.AddColumn()
	resNode.AddColumn()

	lastSubset := ""
	subsetNode := resNode
	for _, r := range t.Status.SortedResources() {
		if lastSubset != r.Subset {
			if r.Subset == "" {
				subsetNode = resNode
			} else {
				resourceSetName := resourceset.T{
					Name:        r.Subset,
					DriverGroup: r.ResourceID.DriverGroup(),
				}.String()
				subsetNode = resNode.AddNode()
				subsetNode.AddColumn().AddText(resourceSetName)
				subsetNode.AddColumn()
				subsetNode.AddColumn()
				parallel := ""
				if t.Config.ActorConfig != nil {
					if subset, ok := t.Config.Subsets[resourceSetName]; ok {
						if subset.Parallel {
							parallel = "//"
						}
					}
				}
				subsetNode.AddColumn().AddText(parallel)
			}
			lastSubset = r.Subset
		}
		doResource := func(n *tree.Node, r resource.Status) *tree.Column {
			flags := ResourceFlagsString(r.ResourceID.Name, t, r)
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
			return desc
		}
		n := subsetNode.AddNode()
		lastDesc := doResource(n, r)
		if encapStatus, ok := t.Status.Encap[r.ResourceID.Name]; ok {
			var l []string
			if encapStatus.IsFrozen() {
				l = append(l, rawconfig.Colorize.Frozen("frozen"))
			}
			if len(l) > 0 {
				lastDesc.AddText(strings.Join(l, " "))
			}
			rids := xmap.Keys(encapStatus.Resources)
			slices.Sort(rids)
			for _, r := range encapStatus.SortedResources() {
				encapNode := n.AddNode()
				doResource(encapNode, r)
			}
		}
	}
	t.loadTreeNodeParents(head)
	t.loadTreeNodeChildren(head)
}

func (t States) descString() string {
	l := make([]string, 0)

	if t.Config.ActorConfig != nil {
		// Overall
		if t.Status.Overall == status.Warn {
			l = append(l, colorstatus.Sprint(t.Status.Overall, rawconfig.Colorize))
		}

		// Frozen
		if !t.Status.FrozenAt.IsZero() {
			l = append(l, rawconfig.Colorize.Frozen("frozen"))
		}

		// Node frozen
		if !t.Node.FrozenAt.IsZero() {
			l = append(l, rawconfig.Colorize.Frozen("node-frozen"))
		}

		// Provisioned
		switch t.Status.Provisioned {
		case provisioned.False:
			l = append(l, rawconfig.Colorize.Error("not-provisioned"))
		case provisioned.Mixed:
			l = append(l, rawconfig.Colorize.Error("mix-provisioned"))
		}
	}

	if !t.Config.UpdatedAt.IsZero() {
		// Priority
		if s := t.Config.Priority.StatusString(); s != "" {
			l = append(l, rawconfig.Colorize.Secondary(s))
		}
	}

	// Monitor state
	if t.Monitor.UpdatedAt.IsZero() {
		// Daemon down
		// *or* imon deleted
		// *or* imon not yet started
		if t.Monitor.UpdatedAt.IsZero() {
			l = append(l, rawconfig.Colorize.Warning("no-monitor"))
		}
	} else {
		switch t.Monitor.State {
		case MonitorStateIdle:
			l = append(l, rawconfig.Colorize.Secondary(t.Monitor.State.String()))
		default:
			l = append(l, rawconfig.Colorize.Primary(t.Monitor.State.String()))
		}

		// Monitor global expect
		switch t.Monitor.GlobalExpect {
		case MonitorGlobalExpectNone:
		case MonitorGlobalExpectInit:
		default:
			l = append(l, t.Monitor.GlobalExpect.String())
		}

		// Monitor local expect
		switch t.Monitor.LocalExpect {
		case MonitorLocalExpectNone:
		case MonitorLocalExpectInit:
		default:
			l = append(l, rawconfig.Colorize.Secondary(t.Monitor.LocalExpect.String()))
		}
	}

	return strings.Join(l, " ")
}

func (t States) loadTreeNodeParents(head *tree.Node) {
	if len(t.Monitor.Parents) == 0 {
		return
	}
	n := head.AddNode()
	n.AddColumn().AddText("parents")
	for relation, availStatus := range t.Monitor.Parents {
		pNode := n.AddNode()
		pNode.AddColumn().AddText(relation).SetColor(rawconfig.Color.Bold)
		pNode.AddColumn()
		pNode.AddColumn().AddText(colorstatus.Sprint(availStatus, rawconfig.Colorize))
	}
}

func (t States) loadTreeNodeChildren(head *tree.Node) {
	if len(t.Monitor.Children) == 0 {
		return
	}
	n := head.AddNode()
	n.AddColumn().AddText("children")
	for relation, availStatus := range t.Monitor.Children {
		pNode := n.AddNode()
		pNode.AddColumn().AddText(relation).SetColor(rawconfig.Color.Bold)
		pNode.AddColumn()
		pNode.AddColumn().AddText(colorstatus.Sprint(availStatus, rawconfig.Colorize))
	}
}
