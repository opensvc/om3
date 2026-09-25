package object

import (
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/key"
)

// capOptions is the pg keyword capping each compute claim type.
var capOptions = map[string]string{
	claim.TypeCPU:    "pg_cpu_quota",
	claim.TypeMemory: "pg_mem_limit",
}

// takesDefaultCaps is true for a resource running a container, which is
// given the caps its namespace claims by default when nothing caps it.
//
// A container is what a namespace user deploys without knowing the node it
// lands on, so its caps are the namespace's to decide when it says none. A
// process of an app resource is left alone: it runs a command of the object,
// whose caps are the object's to say.
func takesDefaultCaps(group driver.Group, typ string) bool {
	switch group {
	case driver.GroupContainer:
		return true
	case driver.GroupTask:
		switch typ {
		case "podman", "docker", "oci":
			return true
		}
	}
	return false
}

// runsProcesses is true for a resource whose processes are placed in its pg,
// and consume what its caps admit.
func runsProcesses(group driver.Group) bool {
	switch group {
	case driver.GroupApp, driver.GroupContainer, driver.GroupTask:
		return true
	}
	return false
}

// capAs is the cap a section says of a compute claim type, as the node given
// evaluates it, and whether it says one.
func (t *core) capAs(section, claimType, nodename string) (int64, bool, error) {
	s, err := t.config.EvalNoConvAs(key.New(section, capOptions[claimType]), nodename)
	if err != nil || s == "" {
		return 0, false, nil
	}
	v, err := claim.Parse(claimType, s, runtime.NumCPU())
	if err != nil {
		return 0, false, fmt.Errorf("%s.%s: %w", section, capOptions[claimType], err)
	}
	return v, true, nil
}

// subsetSection is the section holding the keywords of the subset a
// resource is in, and empty for a resource in none.
func (t *core) subsetSection(rid string, group driver.Group) string {
	name, _ := t.config.EvalNoConv(key.New(rid, "subset"))
	if name == "" {
		return ""
	}
	if s := "subset#" + group.String() + ":" + name; t.config.HasSectionString(s) {
		return s
	}
	return "subset#" + name
}

// defaultCapAs is the cap the namespace gives a resource on a compute claim
// type, as the node given evaluates it, and whether it gives one.
//
// It is given to a container of a namespace claiming the type with a default,
// when neither the container, its subset nor its object says a cap: a cap
// above it already bounds it.
func (t *core) defaultCapAs(rid, claimType, nodename string) (string, bool, error) {
	if t.path.Namespace == naming.NsRoot {
		return "", false, nil
	}
	id, err := resourceid.Parse(rid)
	if err != nil {
		return "", false, nil
	}
	group := id.DriverGroup()
	if !takesDefaultCaps(group, t.config.GetString(key.New(rid, "type"))) {
		return "", false, nil
	}
	for _, section := range []string{rid, t.subsetSection(rid, group), "DEFAULT"} {
		if section == "" {
			continue
		}
		if s, _ := t.config.EvalNoConvAs(key.New(section, capOptions[claimType]), nodename); s != "" {
			return "", false, nil
		}
	}
	return claim.Default(t.path.Namespace, claimType)
}

// resourceCapAs is the cap of a resource on a compute claim type, as the
// node given evaluates it: its own, else the default of its namespace, else
// none, which is Unbounded.
func (t *core) resourceCapAs(rid, claimType, nodename string) (int64, error) {
	if v, ok, err := t.capAs(rid, claimType, nodename); err != nil || ok {
		return v, err
	}
	s, ok, err := t.defaultCapAs(rid, claimType, nodename)
	if err != nil || !ok {
		return claim.Unbounded, err
	}
	v, err := claim.Parse(claimType, s, runtime.NumCPU())
	if err != nil {
		return 0, fmt.Errorf("%s claim default of the %s namespace: %w", claimType, t.path.Namespace, err)
	}
	return v, nil
}

func capMin(a, b int64) int64 {
	switch {
	case a == claim.Unbounded:
		return b
	case b == claim.Unbounded:
		return a
	default:
		return min(a, b)
	}
}

func capSum(a, b int64) int64 {
	if a == claim.Unbounded || b == claim.Unbounded {
		return claim.Unbounded
	}
	return a + b
}

// instanceClaimAs is what an instance of the object takes of a compute claim
// type on the node given: what its processes are capped to, each group
// bounded by the smaller of its own cap and what the groups nested in it
// add up to.
//
// With standbyOnly, it counts the standby resources alone, which are what an
// instance runs on a node where the object is not started.
func (t *core) instanceClaimAs(claimType, nodename string, standbyOnly bool) (int64, error) {
	bySubset := make(map[string]int64)
	for _, rid := range t.config.SectionStrings() {
		id, err := resourceid.Parse(rid)
		if err != nil {
			continue
		}
		group := id.DriverGroup()
		if !runsProcesses(group) {
			continue
		}
		if t.boolAs(rid, "disable", nodename) || t.boolAs(rid, "encap", nodename) {
			continue
		}
		if standbyOnly && !t.boolAs(rid, "standby", nodename) {
			continue
		}
		v, err := t.resourceCapAs(rid, claimType, nodename)
		if err != nil {
			return 0, err
		}
		subset := t.subsetSection(rid, group)
		if sum, ok := bySubset[subset]; ok {
			bySubset[subset] = capSum(sum, v)
		} else {
			bySubset[subset] = v
		}
	}
	total := int64(0)
	for subset, v := range bySubset {
		if subset != "" {
			c, ok, err := t.capAs(subset, claimType, nodename)
			if err != nil {
				return 0, err
			}
			if ok {
				v = capMin(v, c)
			}
		}
		total = capSum(total, v)
	}
	if len(bySubset) == 0 {
		// Nothing runs, so the cap of the object bounds nothing.
		return 0, nil
	}
	c, ok, err := t.capAs("DEFAULT", claimType, nodename)
	if err != nil {
		return 0, err
	}
	if ok {
		total = capMin(total, c)
	}
	return total, nil
}

func (t *core) boolAs(section, option, nodename string) bool {
	s, _ := t.config.EvalNoConvAs(key.New(section, option), nodename)
	v, _ := strconv.ParseBool(s)
	return v
}

// ComputeClaims is what the object claims of each compute type, cluster-wide:
// what its processes are capped to, on every instance it may run at once.
//
// A failover object runs one started instance, and a flex object as many as
// its target. Every other instance runs its standby resources. The instances
// started are counted where they take the most, since any node may be the one
// running them: the claim is what the object may take, not what it takes now.
//
// A process capped by nothing takes what it finds, and makes the claim of
// the object Unbounded on that type.
func (t *core) ComputeClaims() (map[string]int64, error) {
	nodes, err := t.Nodes()
	if err != nil {
		return nil, err
	}
	started := 1
	if t.Topology() == topology.Flex {
		started = t.flexTargetOf(len(nodes))
	}
	m := make(map[string]int64)
	for _, claimType := range claim.ComputeTypes {
		total := int64(0)
		deltas := make([]int64, 0, len(nodes))
		for _, nodename := range nodes {
			full, err := t.instanceClaimAs(claimType, nodename, false)
			if err != nil {
				return nil, err
			}
			standby, err := t.instanceClaimAs(claimType, nodename, true)
			if err != nil {
				return nil, err
			}
			total = capSum(total, standby)
			if full == claim.Unbounded {
				deltas = append(deltas, claim.Unbounded)
			} else if standby != claim.Unbounded {
				deltas = append(deltas, full-standby)
			}
		}
		// The largest first, Unbounded above all.
		slices.SortFunc(deltas, func(a, b int64) int {
			switch {
			case a == b:
				return 0
			case a == claim.Unbounded:
				return -1
			case b == claim.Unbounded:
				return 1
			case a > b:
				return -1
			default:
				return 1
			}
		})
		for i := 0; i < started && i < len(deltas); i++ {
			total = capSum(total, deltas[i])
		}
		m[claimType] = total
	}
	return m, nil
}

// flexTargetOf is the flex target of the object for its count of nodes,
// bounded the way the daemon bounds it.
//
// It is not read with FlexTarget, which bounds it by the peers of this node:
// a claim is weighed on the node handling the write, which need not be one
// of the nodes of the object.
func (t *core) flexTargetOf(count int) int {
	lowest := 0
	if t.path.Kind == naming.KindSvc {
		lowest = 1
	}
	clamp := func(v, low, high int) int {
		return max(low, min(v, high))
	}
	flexMin := lowest
	if v, err := t.config.GetIntStrict(key.Parse("flex_min")); err == nil {
		flexMin = clamp(v, lowest, count)
	}
	flexMax := count
	if v, err := t.config.GetIntStrict(key.Parse("flex_max")); err == nil {
		flexMax = clamp(v, flexMin, count)
	}
	if v, err := t.config.GetIntStrict(key.Parse("flex_target")); err == nil {
		return clamp(v, flexMin, flexMax)
	}
	return flexMin
}

// ComputeClaimer is an object claiming compute of its namespace.
type ComputeClaimer interface {
	ComputeClaims() (map[string]int64, error)
}

// DescribeComputeClaims says what an object claims, in the words of the
// units, for a message.
func DescribeComputeClaims(m map[string]int64) string {
	l := make([]string, 0, len(m))
	for _, claimType := range claim.ComputeTypes {
		if v, ok := m[claimType]; ok {
			l = append(l, claimType+" "+claim.Format(claimType, v))
		}
	}
	return strings.Join(l, ", ")
}
