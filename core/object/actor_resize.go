package object

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// ResizeStep is one link of the chain a resize walks, and the size that
	// link is asked to reach.
	ResizeStep struct {
		// Path names the object the resource belongs to, and is empty for
		// the object the resize was asked of. A chain can cross into another
		// object: a volume resource stands for the head of its volume.
		Path    string `json:"path,omitempty"`
		RID     string `json:"rid"`
		Driver  string `json:"driver"`
		From    int64  `json:"from"`
		To      int64  `json:"to"`
		Comment string `json:"comment,omitempty"`

		// Skip says the link already holds what the link above it needs, so
		// there is nothing to do to it. It is listed so a plan shows the
		// whole chain.
		Skip bool `json:"skip,omitempty"`

		// Stage is the step of the chain this link is grown in. A chain
		// crossing no replicated resource has one stage, numbered 0.
		Stage int `json:"stage"`

		// r is the resource the step changes. A rid alone does not name it,
		// because the chain can cross into another object.
		r resource.Driver

		// owner is the object holding the configuration the size was asked
		// for in.
		owner *actor
	}

	// ResizePlan is what a resize would do, in the order it would do it.
	ResizePlan struct {
		// Steps are ordered as they would be applied: a grow works up from
		// the device to the filesystem, a shrink works down from the
		// filesystem to the device.
		Steps []ResizeStep `json:"steps"`

		// IsShrink says the chain is being made smaller, which is the
		// direction that destroys data when it is applied in the wrong order.
		IsShrink bool `json:"is_shrink"`

		// Stages is how many stages the chain is grown in, which is one more
		// than the number of replicated resources it crosses. Every node has
		// to finish a stage before any node starts the next one: a replicated
		// resource offers only what its smallest replica holds.
		Stages int `json:"stages"`
	}

	// ResizeOptions tunes a resize.
	ResizeOptions struct {
		// SkipHeadStage leaves the last stage of the chain alone. It holds
		// the head, which grows only where the object is up, so a node that
		// does not hold it up asks for the stages below it and is told there
		// is no stage of that number to run here when it reaches the last.
		SkipHeadStage bool

		// GrowOnly makes a resize that would shrink do nothing, instead of
		// doing it or refusing it. It is what an orchestration converging
		// every node to a size wants: a node already holding more than that
		// has nothing to do, and saying so is not the same as failing.
		GrowOnly bool

		// Force allows a resize that would grow one replica of a replicated
		// object on its own. The orchestration passes it for the phase that
		// runs once every node has grown what is under the replicated
		// resource.
		Force bool
	}

	// resizeLevel is the resources a chain rests on at one depth, all asked
	// for the same size. An array is the reason there can be several: it
	// rests on each of its members, and asks the same of each.
	resizeLevel []resizeLink

	// resizeLink is one resource of a resize chain, with the object it
	// belongs to, because a chain can cross into another object.
	resizeLink struct {
		path naming.Path
		r    resource.Driver

		// owner is the object the resource belongs to, so the size a link
		// reaches can be written back where it was asked for, even when the
		// chain has crossed into another object.
		owner *actor
	}
)

func (t ResizeStep) String() string {
	rid := t.RID
	if t.Path != "" {
		rid += " (" + t.Path + ")"
	}
	s := fmt.Sprintf("%-30s %-18s %10s -> %-10s",
		rid, t.Driver,
		sizeconv.BSizeCompact(float64(t.From)),
		sizeconv.BSizeCompact(float64(t.To)))
	if t.Comment != "" {
		s += "  " + t.Comment
	}
	return strings.TrimRight(s, " ")
}

func (t ResizePlan) String() string {
	direction := "grow"
	if t.IsShrink {
		direction = "shrink"
	}
	if len(t.Steps) == 0 {
		return "nothing to do: the chain has no link to change"
	}
	if !t.HasWork() {
		return "nothing to do: every link already holds the size asked of it"
	}
	lines := make([]string, 0, len(t.Steps)+t.Stages+1)
	lines = append(lines, fmt.Sprintf("%s, in this order:", direction))
	indent := "  "
	if t.Stages > 1 {
		indent = "    "
	}
	stage := -1
	for _, step := range t.Steps {
		if t.Stages > 1 && step.Stage != stage {
			stage = step.Stage
			lines = append(lines, "  "+stageTitle(stage, t.IsShrink))
		}
		lines = append(lines, indent+step.String())
	}
	return strings.Join(lines, "\n")
}

// stageTitle names a stage and says what separates it from the one before,
// which is what an operator has to know before running one by hand.
func stageTitle(stage int, isShrink bool) string {
	if stage == 0 && !isShrink {
		return "stage 0, on every node:"
	}
	if isShrink {
		return fmt.Sprintf("stage %d, once every node has finished the stage above:", stage)
	}
	return fmt.Sprintf("stage %d, once every node has finished stage %d:", stage, stage-1)
}

// HasWork says the plan changes something. A plan of links that all hold
// enough already changes nothing.
func (t ResizePlan) HasWork() bool {
	for _, step := range t.Steps {
		if !step.Skip {
			return true
		}
	}
	return false
}

// resizeChain returns the resources a resize of r has to change, from r down
// to the deepest one it rests on.
//
// A link is found by asking the resource what devices it sits on, and the
// object which resource exposes each of them. The walk stops where no resource
// of the object exposes the device: below that the object owns nothing.
//
// A link that stands for a resource of another object continues the walk
// there, from that object's head.
//
// seen holds the links already walked, keyed by object and rid, so a chain
// that loops back into itself is refused rather than walked forever.
func (t *actor) resizeChain(ctx context.Context, r resource.Driver, seen map[string]bool) ([]resizeLevel, error) {
	first := resizeLink{path: t.path, r: r, owner: t}
	if seen[resizeLinkKey(t.path, r.RID())] {
		return nil, fmt.Errorf("%s %s is reached twice: a resize of a chain that loops is not supported", t.path, r.RID())
	}
	seen[resizeLinkKey(t.path, r.RID())] = true
	levels := []resizeLevel{{first}}
	for {
		last := levels[len(levels)-1]

		// A resource that stands for a resource of another object, like a
		// volume resource standing for the head of its volume, holds no size
		// of its own. The chain continues in that object, and this link drops
		// out of it: there is nothing here to change.
		if len(last) == 1 {
			if target, ok := last[0].r.(resource.ResizeTargeter); ok {
				p, err := target.ResizeTarget(ctx)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", last[0].r.RID(), err)
				}
				sub, err := headResizeChain(ctx, p, seen)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", last[0].r.RID(), err)
				}
				return append(levels[:len(levels)-1], sub...), nil
			}
		}

		next, err := t.resizeLevelBelow(ctx, last, seen)
		if err != nil {
			return nil, err
		}
		if len(next) == 0 {
			return levels, nil
		}
		levels = append(levels, next)
	}
}

// resizeLevelBelow returns the resources a level rests on, which is every
// resource any of its links rests on.
func (t *actor) resizeLevelBelow(ctx context.Context, level resizeLevel, seen map[string]bool) (resizeLevel, error) {
	var next resizeLevel
	add := func(r resource.Driver) {
		key := resizeLinkKey(t.path, r.RID())
		if seen[key] {
			return
		}
		seen[key] = true
		next = append(next, resizeLink{path: t.path, r: r, owner: t})
	}
	for _, link := range level {
		// A link that rests on something no device leads to names it, and
		// the resource answering to that name is below it.
		if namer, ok := link.r.(resource.ResizeRestsOn); ok {
			if below := t.resizeProvider(ctx, namer.ResizeRestsOn(ctx)); below != nil {
				add(below)
				continue
			}
		}
		sub, ok := link.r.(resource.SubDeviceser)
		if !ok {
			continue
		}
		for _, dev := range sub.SubDevices(ctx) {
			below, err := t.ResourceHandlingDevice(ctx, dev)
			if err != nil {
				return nil, err
			}
			if below == nil {
				continue
			}
			add(below)
		}
	}
	sort.Slice(next, func(i, j int) bool { return next[i].r.RID() < next[j].r.RID() })
	return next, nil
}

// replicatedResizeResource returns the resource of the object whose size is
// replicated to peers, and nil when the object has none.
// replicatedResizeResourceCount counts the resources of the object whose size
// is replicated to peers, which is how many stage boundaries a chain of them
// has.
//
// It reads driver properties only, so it answers on a node that cannot touch
// the devices, which is what a node asking whether a stage is its to run
// needs. The count of the chain itself is the authority, and a chain crossing
// into another object can hold a boundary this does not see.
func (t *actor) replicatedResizeResourceCount() int {
	n := 0
	for _, r := range t.Resources() {
		if i, ok := r.(resource.ResizeIsReplicated); ok && i.ResizeIsReplicated() {
			n++
		}
	}
	return n
}

func (t *actor) replicatedResizeResource(ctx context.Context) resource.Driver {
	for _, r := range t.Resources() {
		if i, ok := r.(resource.ResizeIsReplicated); ok && i.ResizeIsReplicated() {
			return r
		}
	}
	return nil
}

// ResizePlanStage works out what growing one stage of the chain to the given
// size would do, and changes nothing.
//
// Stage 0 is walked from the first replicated link rather than from the head,
// because the node running it may not hold the object up, and a node that
// does not cannot read the head: its filesystem is not mounted there. The
// replicated link is asked for the size the object is asked to be, which is
// what the head asks of it when the head is a filesystem taking the whole
// device. Every later stage rests on the head being readable, and runs where
// it is.
//
// The links of the other stages are listed, so a plan shows the whole chain,
// and marked as nothing to do here.
func (t *actor) ResizePlanStage(ctx context.Context, rid string, to int64, stage int, opts ResizeOptions) (ResizePlan, error) {
	var plan ResizePlan
	if stage < 0 {
		return plan, fmt.Errorf("a stage is numbered from 0")
	}
	// Asked before anything is walked, because walking the chain from the head
	// needs the head, and a node that does not hold the object up cannot read
	// it. That node is exactly the one asking whether the stage is its to run.
	stages := t.replicatedResizeResourceCount() + 1
	if stages > instance.MaxResizeStages {
		// The orchestration names the stage each node finished in its monitor
		// state, and there are that many names. A chain crossing more is
		// grown by hand, one --stage at a time.
		return plan, fmt.Errorf("%s crosses %d replicated resources, so it grows in %d stages, and an orchestration grows a chain in at most %d", t.path, stages-1, stages, instance.MaxResizeStages)
	}
	if stage >= stages {
		return plan, fmt.Errorf("%s grows in %d stage(s), numbered 0 to %d: %w", t.path, stages, stages-1, xerrors.ResizeNoSuchStage)
	}
	if opts.SkipHeadStage && stage == stages-1 {
		return plan, fmt.Errorf("%s stage %d holds the head, which grows where the object is up: %w", t.path, stage, xerrors.ResizeNoSuchStage)
	}

	change := sizeconv.Change{Value: to}
	r := t.replicatedResizeResource(ctx)
	switch {
	case stage > 0 || r == nil:
		// Every stage but the first is of the chain under the head, and so is
		// a chain crossing nothing replicated, which is the single stage 0.
		var err error
		plan, err = t.ResizePlan(ctx, rid, change, opts)
		if err != nil {
			return plan, err
		}
	default:
		chain, err := t.resizeChain(ctx, r, map[string]bool{})
		if err != nil {
			return plan, err
		}
		plan, err = buildResizePlan(ctx, chain, change, t.path, opts)
		if err != nil {
			return plan, err
		}
	}
	if stage >= plan.Stages {
		if plan.Stages == 1 {
			return plan, fmt.Errorf("%s grows in a single stage, numbered 0: %w", t.path, xerrors.ResizeNoSuchStage)
		}
		return plan, fmt.Errorf("%s grows in %d stages, numbered 0 to %d: %w", t.path, plan.Stages, plan.Stages-1, xerrors.ResizeNoSuchStage)
	}
	if opts.SkipHeadStage && stage == plan.Stages-1 {
		// The last stage holds the head, and a head grows where the object is
		// up. A node that does not hold it up has nothing left to do.
		return plan, fmt.Errorf("%s stage %d holds the head, which grows where the object is up: %w", t.path, stage, xerrors.ResizeNoSuchStage)
	}
	for i := range plan.Steps {
		if plan.Steps[i].Stage == stage {
			continue
		}
		plan.Steps[i].Skip = true
		plan.Steps[i].To = plan.Steps[i].From
		plan.Steps[i].Comment = fmt.Sprintf("grown in stage %d", plan.Steps[i].Stage)
	}
	t.localizeResizePlan(&plan)
	return plan, nil
}

// ResizeStage grows one stage of the chain to the given size, and leaves the
// other stages alone.
func (t *actor) ResizeStage(ctx context.Context, rid string, to int64, stage int, opts ResizeOptions) error {
	plan, err := t.ResizePlanStage(ctx, rid, to, stage, opts)
	if err != nil {
		return err
	}
	return t.applyResizePlan(ctx, plan, fmt.Sprintf("stage %d", stage))
}

// refuseLoneReplicaResize stops a resize that would run every stage of a
// chain on one node.
//
// A chain grows in as many stages as the replicated resources it crosses,
// plus one, and every node has to finish a stage before any node starts the
// next: a replicated resource offers only what its smallest replica holds.
// Running them all here strands the space: the object gains nothing, the
// nodes stop matching, and the size written back records one no peer has.
//
// Running one stage is not stopped. That is what an orchestrated resize does
// on each node, and what --stage does by hand.
func refuseLoneReplicaResize(plan ResizePlan, home naming.Path, opts ResizeOptions) error {
	if opts.Force || plan.Stages < 2 {
		return nil
	}
	return fmt.Errorf("%s grows in %d stages, and this would run them all on %s: a replicated resource offers what its smallest replica holds, so the space would be stranded. Use \"om %s resize\" to grow every node, --stage to run one stage here, or --force to run them all anyway",
		home, plan.Stages, hostname.Hostname(), home)
}

// resizeProvider returns the resource of the object answering to a name, and
// nil when the name is empty or nothing answers to it.
func (t *actor) resizeProvider(ctx context.Context, name string) resource.Driver {
	if name == "" {
		return nil
	}
	for _, r := range t.Resources() {
		provider, ok := r.(resource.ResizeProvides)
		if !ok {
			continue
		}
		if provider.ResizeProvides(ctx) == name {
			return r
		}
	}
	return nil
}

// resizeChainOf returns the chain a resize of rid walks.
func (t *actor) resizeChainOf(ctx context.Context, rid string, seen map[string]bool) ([]resizeLevel, error) {
	t.ConfigureResources()
	r := t.ResourceByID(rid)
	if r == nil {
		return nil, fmt.Errorf("%s has no %s resource", t.path, rid)
	}
	return t.resizeChain(ctx, r, seen)
}

// headResizeChain returns the chain a resize asked of an object itself walks,
// starting at the resource that object exposes to its consumers.
func headResizeChain(ctx context.Context, p naming.Path, seen map[string]bool) ([]resizeLevel, error) {
	type headResizer interface {
		HeadRID(context.Context) (string, error)
		resizeChainOf(ctx context.Context, rid string, seen map[string]bool) ([]resizeLevel, error)
	}
	o, err := New(p)
	if err != nil {
		return nil, err
	}
	i, ok := o.(headResizer)
	if !ok {
		return nil, fmt.Errorf("%s exposes no resource to resize", p)
	}
	rid, err := i.HeadRID(ctx)
	if err != nil {
		return nil, err
	}
	return i.resizeChainOf(ctx, rid, seen)
}

func resizeLinkKey(p naming.Path, rid string) string {
	return p.String() + " " + rid
}

// ResizePlan works out what resizing rid to the given size would do, and
// changes nothing.
//
// Every link is asked, so a chain holding one link that cannot do it is
// refused whole. A link answers the size it needs from the link below, which
// is not always the size it was asked for: a raid6 md holding n devices needs
// to(n-2) from each.
func (t *actor) ResizePlan(ctx context.Context, rid string, change sizeconv.Change, opts ResizeOptions) (ResizePlan, error) {
	var plan ResizePlan

	// The rid is a selector, and a resize is of one resource: which of
	// several a size belongs to is not something this can decide.
	selected := resourceselector.New(t, resourceselector.WithRID(rid)).Resources()
	switch len(selected) {
	case 0:
		return plan, fmt.Errorf("no resource selected by %s", rid)
	case 1:
	default:
		rids := make([]string, len(selected))
		for i, r := range selected {
			rids[i] = r.RID()
		}
		return plan, fmt.Errorf("%s selects %s: a resize is asked of one resource",
			rid, strings.Join(rids, ", "))
	}
	chain, err := t.resizeChain(ctx, selected[0], map[string]bool{})
	if err != nil {
		return plan, err
	}
	plan, err = buildResizePlan(ctx, chain, change, t.path, opts)
	if err != nil {
		return plan, err
	}

	t.localizeResizePlan(&plan)
	return plan, nil
}

// localizeResizePlan drops the object name from the steps of the object the
// resize was asked of: that one goes without saying. What stays named is what
// the chain crossed into.
func (t *actor) localizeResizePlan(plan *ResizePlan) {
	for i := range plan.Steps {
		if plan.Steps[i].Path == t.path.String() {
			plan.Steps[i].Path = ""
		}
	}
}

// resizeStages says which stage each level of a chain is grown in.
//
// A replicated link is a boundary. Everything under it has to be grown on
// every node before it can be grown itself, because a replicated resource
// offers only what its smallest replica holds. So the levels below the first
// boundary are stage 0, that boundary and the levels above it up to the next
// one are stage 1, and so on. A chain crossing no replicated resource has the
// single stage 0.
//
// The stage belongs to the level rather than to the link, because a barrier
// synchronises the whole chain: a link sharing a level with a replicated one
// is grown after the same barrier anyway, and waiting a stage longer than it
// strictly has to costs nothing.
//
// The returned slice is indexed like the chain, so index 0 is the head.
func resizeStages(chain []resizeLevel) []int {
	stages := make([]int, len(chain))
	stage := 0
	for i := len(chain) - 1; i >= 0; i-- {
		if levelIsReplicated(chain[i]) {
			stage++
		}
		stages[i] = stage
	}
	return stages
}

// levelIsReplicated says whether any link of a level replicates its size to
// peers, which makes the level a stage boundary.
func levelIsReplicated(level resizeLevel) bool {
	for _, link := range level {
		if i, ok := link.r.(resource.ResizeIsReplicated); ok && i.ResizeIsReplicated() {
			return true
		}
	}
	return false
}

// buildResizePlan works out what resizing a chain would do, and changes
// nothing. The chain runs from the resource the size was asked of down to the
// deepest one it rests on.
// home is the object the resize was asked of, so a link of another object is
// named with it.
func buildResizePlan(ctx context.Context, chain []resizeLevel, change sizeconv.Change, home naming.Path, opts ResizeOptions) (ResizePlan, error) {
	var plan ResizePlan
	if len(chain) == 0 || len(chain[0]) == 0 {
		return plan, fmt.Errorf("nothing to resize")
	}
	head := chain[0][0]

	// The size asked for is of the resource named, so the direction is read
	// from that one. The levels below follow it.
	sizer, ok := head.r.(resource.Sizer)
	if !ok {
		return plan, resizeRefusal(head, head, home, "reports no size")
	}
	from, err := sizer.CurrentSize(ctx)
	if err != nil {
		return plan, fmt.Errorf("%s: size: %w", resizeLinkName(head, home), err)
	}
	to := change.Resolve(from)
	if to <= 0 {
		return plan, fmt.Errorf("%s: %s of %s leaves nothing", resizeLinkName(head, home), change, sizeconv.BSizeCompact(float64(from)))
	}
	plan.IsShrink = to < from
	if plan.IsShrink && opts.GrowOnly {
		// Already larger than asked for. Saying there is nothing to do is
		// the answer here, not shrinking it back and not refusing.
		return plan, nil
	}

	stages := resizeStages(chain)
	// The chain is indexed from the head, which is in the last stage.
	plan.Stages = stages[0] + 1

	levelSteps := make([][]ResizeStep, 0, len(chain))
	for levelIndex, level := range chain {
		// Every resource of a level is asked for the same size, and the
		// level below has to satisfy the most demanding of them.
		var needBelow int64
		var steps []ResizeStep
		for _, link := range level {
			sizer, ok := link.r.(resource.Sizer)
			if !ok {
				return plan, resizeRefusal(link, head, home, "reports no size")
			}
			resizer, ok := link.r.(resource.Resizer)
			if !ok {
				return plan, resizeRefusal(link, head, home, "cannot resize")
			}
			linkFrom, err := sizer.CurrentSize(ctx)
			if err != nil {
				return plan, fmt.Errorf("%s: size: %w", resizeLinkName(link, home), err)
			}
			linkNeedBelow, err := resizer.ResizePlan(ctx, to)
			if err != nil {
				return plan, fmt.Errorf("%s: %w", resizeLinkName(link, home), err)
			}
			if linkNeedBelow > needBelow {
				needBelow = linkNeedBelow
			}
			step := ResizeStep{
				Path:   link.path.String(),
				RID:    link.r.RID(),
				Driver: driverOf(link.r),
				From:   linkFrom,
				To:     to,
				Stage:  stages[levelIndex],
				r:      link.r,
				owner:  link.owner,
			}

			// A link below only has to be large enough. One that already is
			// is left alone: shrinking it back would be work nobody asked
			// for, and it would put a shrink in the middle of a grow, which
			// has no safe order. This is also what heals a chain left uneven
			// by a resize that failed part way.
			if (!plan.IsShrink && linkFrom >= to) || (plan.IsShrink && linkFrom <= to) {
				step.Skip = true
				step.To = linkFrom
				step.Comment = "already holds what the link above needs"
			} else if linkNeedBelow != to {
				step.Comment = fmt.Sprintf("asks %s of the link below", sizeconv.BSizeCompact(float64(linkNeedBelow)))
			}
			steps = append(steps, step)
		}
		levelSteps = append(levelSteps, steps)
		to = needBelow
	}

	// A chain grows from the bottom up: the space has to exist before
	// anything is stretched onto it. It shrinks from the top down: a
	// filesystem has to give the space back before the device under it is
	// taken away, or what is still mounted is larger than what holds it.
	//
	// The levels are what reverses, not the steps: the resources of one level
	// rest on nothing of each other, so their order among themselves is the
	// order they were found in either way.
	if !plan.IsShrink {
		reverse(levelSteps)
	}
	for _, steps := range levelSteps {
		plan.Steps = append(plan.Steps, steps...)
	}
	return plan, nil
}

// Resize changes the size of rid and of everything it rests on.
func (t *actor) Resize(ctx context.Context, rid string, change sizeconv.Change, opts ResizeOptions) error {
	plan, err := t.ResizePlan(ctx, rid, change, opts)
	if err != nil {
		return err
	}
	if err := refuseLoneReplicaResize(plan, t.path, opts); err != nil {
		return err
	}
	return t.applyResizePlan(ctx, plan, rid)
}

// applyResizePlan changes what the plan says to change, in the order it says.
func (t *actor) applyResizePlan(ctx context.Context, plan ResizePlan, what string) error {
	if !plan.HasWork() {
		t.log.Infof("resize %s: every link already holds the size asked of it", what)
		return nil
	}
	for _, step := range plan.Steps {
		if step.Skip {
			continue
		}
		resizer, ok := step.r.(resource.Resizer)
		if !ok {
			// The plan said otherwise a moment ago.
			return fmt.Errorf("%s: cannot be resized", step.RID)
		}
		name := step.RID
		if step.Path != "" {
			name += " (" + step.Path + ")"
		}
		t.log.Infof("resize %s from %s to %s", name,
			sizeconv.BSizeCompact(float64(step.From)),
			sizeconv.BSizeCompact(float64(step.To)))
		if err := resizer.Resize(ctx, step.To); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := step.recordSize(ctx); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// recordSize writes the size a link reached back to the keyword that asked for
// it, so the configuration keeps describing the object.
//
// Without it a chain rebuilt by an unprovision and a provision comes back at
// the size it was first given, silently undoing every resize since, and every
// report reading the configuration is wrong.
//
// Two values are left alone, because both say more than a number does:
//
//   - a policy, like the "100%FREE" a logical volume takes to mean all the
//     space its group has. A resize satisfies it rather than contradicting it,
//     and replacing it with a number is the instruction lost.
//   - a reference, like the "{DEFAULT.size}" a pool-served volume takes to
//     mean the size the volume was claimed with. The value belongs to the
//     keyword pointed at, and that is what an orchestrated resize writes.
//
// A keyword that names no size is left alone too: this keeps a configuration
// accurate, it does not start recording in one that said nothing.
func (t ResizeStep) recordSize(ctx context.Context) error {
	if t.owner == nil {
		return nil
	}
	k := key.T{Section: t.RID, Option: "size"}
	if !t.owner.config.HasKey(k) {
		return nil
	}
	was := t.owner.config.Get(k)

	// A reference says the value lives in another keyword, so that is the one
	// to record in. A pool-served volume points the size of its resources at
	// DEFAULT.size, which is also the size the pool counts the volume as
	// claiming, so leaving it alone lets a resize grow the storage without the
	// cluster ever hearing that the claim grew with it.
	if ref, ok := referencedKey(was); ok {
		k = ref
		was = t.owner.config.Get(k)
	}
	if !isRecordableSize(was) {
		return nil
	}

	// The size reached, not the size asked for. A driver rounds to what it
	// hands out: a zvol to its block size, a logical volume to its extents.
	// Recording the request would put a size in the configuration that the
	// resource does not have.
	sizer, ok := t.r.(resource.Sizer)
	if !ok {
		return nil
	}
	reached, err := sizer.CurrentSize(ctx)
	if err != nil {
		// The resize itself worked. Not being able to read back what it
		// reached is worth saying, and not worth failing for.
		t.owner.log.Infof("%s: size reached cannot be read back, so it is not recorded: %s", t.RID, err)
		return nil
	}
	op := keyop.T{
		Key:   k,
		Op:    keyop.Set,
		Value: sizeconv.ExactBSizeCompact(float64(reached)),
	}
	if op.Value == was {
		return nil
	}
	t.owner.log.Infof("record %s %s -> %s", k, was, op.Value)
	return t.owner.config.Set(op)
}

// referencedKey returns the keyword a value refers to, when the value is
// nothing but a reference to one.
//
// Only a plain "{section.option}" counts. A value mixing a reference with
// anything else says more than where its value lives, and is left alone for
// the same reason a policy is.
func referencedKey(s string) (key.T, bool) {
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return key.T{}, false
	}
	inner := s[1 : len(s)-1]
	if strings.ContainsAny(inner, "{}") {
		return key.T{}, false
	}
	section, option, found := strings.Cut(inner, ".")
	if !found || section == "" || option == "" {
		return key.T{}, false
	}
	return key.T{Section: section, Option: option}, true
}

// isRecordableSize says whether a configured size may be replaced by the size
// a resize reached.
//
// A policy like "100%FREE" and a reference like "{DEFAULT.size}" both say more
// than a number does, and a resize does not contradict either: it satisfies
// the policy, and the reference keeps pointing at the keyword that owns the
// value. Writing a number over them is the instruction lost.
func isRecordableSize(was string) bool {
	return was != "" && !strings.ContainsAny(was, "%{")
}

// resizeRefusal says why a chain cannot be resized. A link that is not the one
// the size was asked of is reported as what holds that one up, so the answer
// is about what the user asked for and not only about where the walk stopped.
func resizeRefusal(link, head resizeLink, home naming.Path, what string) error {
	if link.path == head.path && link.r.RID() == head.r.RID() {
		return fmt.Errorf("%s: its %s driver %s", resizeLinkName(link, home), driverOf(link.r), what)
	}
	return fmt.Errorf("%s rests on %s, whose %s driver %s",
		resizeLinkName(head, home), resizeLinkName(link, home), driverOf(link.r), what)
}

// resizeLinkName names a link, saying which object it belongs to when it is
// not the object the resize was asked of.
func resizeLinkName(link resizeLink, home naming.Path) string {
	if link.path == home {
		return link.r.RID()
	}
	return link.path.String() + " " + link.r.RID()
}

// driverOf names the driver of a resource, for an error to say which driver
// would have to implement a resize. A driver that has not been through its
// manifest has none to name.
func driverOf(r resource.Driver) string {
	m := r.Manifest()
	if m == nil {
		return "unknown"
	}
	return m.DriverID.String()
}

func reverse[T any](l []T) {
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
}
