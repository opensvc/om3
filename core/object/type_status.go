package object

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/core/colorstatus"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/render/tree"
	"github.com/opensvc/om3/util/xmap"
)

type (
	// Digest is a composite extract of different parts of
	// the cluster status.
	Digest struct {
		IsCompat  bool                `json:"is_compat" yaml:"is_compat"`
		Instances instance.StatesList `json:"instances" yaml:"instances"`
		Object    Status              `json:"object" yaml:"object"`
		Path      path.T              `json:"path" yaml:"path"`
	}

	// Status contains the object states obtained via
	// aggregation of all instances states. It exists when an instance config exists somewhere
	Status struct {
		Avail            status.T         `json:"avail" yaml:"avail"`
		FlexTarget       int              `json:"flex_target,omitempty" yaml:"flex_target,omitempty"`
		FlexMin          int              `json:"flex_min,omitempty" yaml:"flex_min,omitempty"`
		FlexMax          int              `json:"flex_max,omitempty" yaml:"flex_max,omitempty"`
		Frozen           string           `json:"frozen" yaml:"frozen"`
		Orchestrate      string           `json:"orchestrate" yaml:"orchestrate"`
		Overall          status.T         `json:"overall" yaml:"overall"`
		PlacementPolicy  placement.Policy `json:"placement_policy" yaml:"placement_policy"`
		PlacementState   placement.State  `json:"placement_state" yaml:"placement_state"`
		Priority         priority.T       `json:"priority" yaml:"priority"`
		Provisioned      provisioned.T    `json:"provisioned" yaml:"provisioned"`
		Scope            []string         `json:"scope" yaml:"scope"`
		Topology         topology.T       `json:"topology" yaml:"topology"`
		UpInstancesCount int              `json:"up_instances_count" yaml:"up_instances_count"`

		UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	}
)

// Render returns a human friendy string representation of the type instance.
func (t Digest) Render(nodes []string) string {
	tree := t.Tree(nodes)
	return tree.Render()
}

// Tree returns a tree loaded with the type instance.
func (t Digest) Tree(nodes []string) *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head(), nodes)
	return tree
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t Digest) LoadTreeNode(head *tree.Node, nodes []string) {
	head.AddColumn().AddText(t.Path.String()).SetColor(rawconfig.Color.Bold)
	head.AddColumn()
	head.AddColumn().AddText(colorstatus.Sprint(t.Object.Avail, rawconfig.Colorize))
	head.AddColumn().AddText(t.descString())
	instances := head.AddNode()
	instances.AddColumn().AddText("instances")
	openMap := make(map[string]any)
	folded := make([]string, 0)
	instMap := t.Instances.ByNode()

	for _, nodename := range nodes {
		if _, ok := instMap[nodename]; ok {
			openMap[nodename] = nil
		}
	}
	for _, nodename := range xmap.Keys(instMap) {
		if _, ok := openMap[nodename]; !ok {
			folded = append(folded, nodename)
		}
	}
	open := xmap.Keys(openMap)
	sort.Sort(sort.StringSlice(open))
	sort.Sort(sort.StringSlice(folded))

	for _, nodename := range folded {
		data := instMap[nodename]
		n := instances.AddNode()
		data.LoadTreeNodeFolded(n)
	}
	for _, nodename := range open {
		data := instMap[nodename]
		n := instances.AddNode()
		data.LoadTreeNode(n)
	}
}

// descString returns a string presenting notable information at the object,
// instances-aggregated, level.
func (t Digest) descString() string {
	l := make([]string, 0)

	// Overall if warn. Else no need to repeat an info we can guess from Avail.
	if t.Object.Overall == status.Warn {
		l = append(l, colorstatus.Sprint(t.Object.Overall, rawconfig.Colorize))
	}

	// Placement
	if t.Object.PlacementState == placement.NonOptimal {
		l = append(l, rawconfig.Colorize.Warning(fmt.Sprintf("%s placement", t.Object.PlacementState)))
	}

	// Agent compatibility
	if !t.IsCompat {
		l = append(l, rawconfig.Colorize.Error("incompatible versions"))
	}

	return strings.Join(l, " ")
}

// NewStatus allocates and return a struct to host an objet full state dataset.
func NewStatus() *Digest {
	t := &Digest{}
	t.Instances = make([]instance.States, 0)
	return t
}

func (s *Status) DeepCopy() *Status {
	return &Status{
		Avail:            s.Avail,
		Overall:          s.Overall,
		Frozen:           s.Frozen,
		Orchestrate:      s.Orchestrate,
		PlacementState:   s.PlacementState,
		PlacementPolicy:  s.PlacementPolicy,
		Provisioned:      s.Provisioned,
		Priority:         s.Priority,
		Topology:         s.Topology,
		FlexTarget:       s.FlexTarget,
		FlexMin:          s.FlexMin,
		FlexMax:          s.FlexMax,
		UpInstancesCount: s.UpInstancesCount,
		Scope:            append([]string{}, s.Scope...),
		UpdatedAt:        s.UpdatedAt,
	}
}
