package object

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/util/device"
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
		// Steps are ordered as they would be applied, which is up from the
		// device to the filesystem: the space has to exist before anything
		// is stretched onto it.
		Steps []ResizeStep `json:"steps"`

		// Stages is how many stages the chain is grown in, which is one more
		// than the number of replicated resources it crosses. Every node has
		// to finish a stage before any node starts the next one: a replicated
		// resource offers only what its smallest replica holds.
		Stages int `json:"stages"`

		// Bottom is the size asked of the bottom of the chain, when the size
		// asked is of the bottom rather than of the resource the resize was
		// asked of, and 0 otherwise. See ResizeOptions.Bottom.
		Bottom int64 `json:"bottom,omitempty"`
	}

	// ResizeOptions tunes a resize.
	ResizeOptions struct {
		// SkipHeadStage leaves the last stage of the chain alone. It holds
		// the head, which grows only where the object is up, so a node that
		// does not hold it up asks for the stages below it and is told there
		// is no stage of that number to run here when it reaches the last.
		SkipHeadStage bool

		// Force allows a resize that would grow one replica of a replicated
		// object on its own. The orchestration passes it for the phase that
		// runs once every node has grown what is under the replicated
		// resource.
		Force bool

		// Bottom says the size asked is the size of the bottom of the chain,
		// rather than of the resource the resize is asked of. It is what a
		// resize asked of a volume means: the size of a volume is what its
		// pool and the claims of its namespace count, which is what the
		// bottom of its chain holds, and the links above it keep a cut of
		// what they rest on for their own metadata, so the head never holds
		// it.
		//
		// The size the head has to reach is worked out from it, up the chain,
		// from what each link hands out of what is below it, and the links
		// are then asked from the top down, as they are for a size of the
		// head: the bottom ends up asked for about the size given. A head
		// already holding what that leaves it has nothing to do, which is
		// what makes asking a volume for the size it is configured to hold
		// harmless.
		//
		// A size recorded in DEFAULT.size is never more than the size given:
		// that is the size the claim of the volume was checked for, and a
		// rounding up on the way down is no reason to outgrow it.
		Bottom bool

		// Configured is the size the volume is configured to hold, for a
		// resize asked of the bottom, and 0 when it says nothing. A size
		// above it that grows nothing is refused with a
		// ResizeGrowsNothingError naming the smallest one that does: asked
		// of the pool through the configuration, it would be claimed with
		// nothing holding it.
		Configured int64
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
	if len(t.Steps) == 0 {
		return "nothing to do: the chain has no link to change"
	}
	if !t.HasWork() {
		return "nothing to do: every link already holds the size asked of it"
	}
	lines := make([]string, 0, len(t.Steps)+t.Stages+1)
	if t.Bottom > 0 {
		lines = append(lines, fmt.Sprintf("grow, for the bottom of the chain to hold %s, in this order:", sizeconv.BSizeCompact(float64(t.Bottom))))
	} else {
		lines = append(lines, "grow, in this order:")
	}
	indent := "  "
	if t.Stages > 1 {
		indent = "    "
	}
	stage := -1
	for _, step := range t.Steps {
		if t.Stages > 1 && step.Stage != stage {
			stage = step.Stage
			lines = append(lines, "  "+stageTitle(stage))
		}
		lines = append(lines, indent+step.String())
	}
	return strings.Join(lines, "\n")
}

// stageTitle names a stage and says what separates it from the one before,
// which is what an operator has to know before running one by hand.
func stageTitle(stage int) string {
	if stage == 0 {
		return "stage 0, on every node:"
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
	return t.resizeChainFromLevel(ctx, resizeLevel{first}, seen)
}

// resizeChainFromLevel walks a chain down from a level already worked out.
func (t *actor) resizeChainFromLevel(ctx context.Context, first resizeLevel, seen map[string]bool) ([]resizeLevel, error) {
	levels := []resizeLevel{first}
	for {
		last := levels[len(levels)-1]

		// A resource that stands for a resource of another object, like a
		// volume resource standing for the head of its volume, holds no size
		// of its own. The chain continues in that object, and this link drops
		// out of it: there is nothing here to change.
		stay, entered, err := t.resizeEnterTargets(ctx, last, seen)
		if err != nil {
			return nil, err
		}
		if len(entered) > 0 {
			// The links that hold a size go on being walked here, and what
			// they reach lines up with what the objects entered reach: a
			// level is grown as one, whichever object each of its links is
			// in.
			merged := entered
			if len(stay) > 0 {
				stayed, err := t.resizeChainFromLevel(ctx, stay, seen)
				if err != nil {
					return nil, err
				}
				merged = mergeResizeLevels(stayed, entered)
			}
			return append(levels[:len(levels)-1], merged...), nil
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

// resizeEnterTargets splits a level into the links that hold a size of their
// own and the chains of the objects the others stand for.
//
// An array over two volumes rests on both, so a level holds as many of these
// as the array has members, and each is a chain of its own object. They are
// merged, because the members of an array are grown together and the levels of
// their chains line up: what each volume exposes is grown at the same depth as
// what the other does.
func (t *actor) resizeEnterTargets(ctx context.Context, level resizeLevel, seen map[string]bool) (resizeLevel, []resizeLevel, error) {
	var (
		stay    resizeLevel
		entered []resizeLevel
	)
	for _, link := range level {
		target, ok := link.r.(resource.ResizeTargeter)
		if !ok {
			stay = append(stay, link)
			continue
		}
		p, err := target.ResizeTarget(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", link.r.RID(), err)
		}
		sub, err := headResizeChain(ctx, p, seen)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", link.r.RID(), err)
		}
		entered = mergeResizeLevels(entered, sub)
	}
	return stay, entered, nil
}

// mergeResizeLevels lines two chains up depth by depth, so that what they hold
// at the same depth is one level, grown together.
func mergeResizeLevels(a, b []resizeLevel) []resizeLevel {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	merged := make([]resizeLevel, n)
	for i := 0; i < n; i++ {
		var level resizeLevel
		if i < len(a) {
			level = append(level, a[i]...)
		}
		if i < len(b) {
			level = append(level, b[i]...)
		}
		sort.Slice(level, func(x, y int) bool {
			if level[x].path.String() != level[y].path.String() {
				return level[x].path.String() < level[y].path.String()
			}
			return level[x].r.RID() < level[y].r.RID()
		})
		merged[i] = level
	}
	return merged
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
// device. With opts.Bottom, that size is worked out from the bottom up to the
// replicated link, which is what it is worked out to up to a filesystem
// taking the whole of it. Every later stage rests on the head being readable,
// and runs where it is.
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
// on each node, and what --stage does by hand. Neither is a plan with nothing
// to do: it strands nothing.
func refuseLoneReplicaResize(plan ResizePlan, home naming.Path, opts ResizeOptions) error {
	if opts.Force || plan.Stages < 2 || !plan.HasWork() {
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
// Every link that has to grow is asked, so a chain holding one link that
// cannot do it is refused whole. A link answers the size it needs from the
// link below, which is not always the size it was asked for: a raid6 md
// holding n devices needs to/(n-2) from each, and a logical volume needs the
// space its volume group has taken plus its growth. The walk stops at a level
// holding what it is asked: the links below it are not asked, and are listed.
// A link asking more than a device no resource handles holds is refused:
// nothing below it grows.
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
//
// The size is of the head, unless opts.Bottom says it is of the bottom, in
// which case the size of the head is worked out from it first.
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
	stages := resizeStages(chain)
	if opts.Bottom {
		bottom, err := resolveBottomSize(ctx, chain, change, home)
		if err != nil {
			return plan, err
		}
		handsUp, err := chainHandsUp(ctx, chain, home)
		if err != nil {
			return plan, err
		}
		to := handsUp(bottom)
		if to <= from && opts.Configured > 0 && bottom > opts.Configured {
			// More than the volume is configured to hold, and nothing grows:
			// the size would be recorded with nothing holding it.
			name := home.String()
			if name == "" {
				name = resizeLinkName(head, home)
			}
			return plan, growsNothingError(ctx, chain, handsUp, name, bottom, from)
		}
		if to <= from {
			// The head holds what the size given leaves it, so nothing
			// above the bottom has to grow. That is the answer to asking a
			// volume for the size it is configured to hold: the
			// configuration records the bottom, which the head never holds.
			//
			// The bottom is still asked the size given, which it holds
			// already but for a bottom a former resize left short of it:
			// the head was worked back down to the bottom it needed, less
			// than the size given by what the links above round off.
			plan.Stages = stages[0] + 1
			plan.Bottom = bottom
			levelSteps := make([][]ResizeStep, 0, len(chain))
			last := len(chain) - 1
			for levelIndex, level := range chain {
				if levelIndex > 0 && levelIndex == last {
					steps, _, err := askResizeLevel(ctx, level, levelIndex, bottom, stages[levelIndex], head, home)
					if err != nil {
						return plan, err
					}
					levelSteps = append(levelSteps, steps)
					continue
				}
				levelSteps = append(levelSteps, notAskedSteps(ctx, level, stages[levelIndex], "holds what the size asked leaves it"))
			}
			reverse(levelSteps)
			for _, steps := range levelSteps {
				plan.Steps = append(plan.Steps, steps...)
			}
			if plan.HasWork() {
				// The bottom grows: the links resting on it are grown
				// onto it. A bottom holding the size changes nothing,
				// the head included.
				unskipSpansBelow(&plan)
			}
			return plan, nil
		}
		plan, err = buildResizePlanTo(ctx, chain, to, home, stages, bottom)
		plan.Bottom = bottom
		return plan, err
	}
	to := change.Resolve(from)
	if to <= 0 {
		return plan, fmt.Errorf("%s: %s of %s leaves nothing", resizeLinkName(head, home), change, sizeconv.BSizeCompact(float64(from)))
	}
	if to < from {
		// A resize only grows. Asking for less is nothing to do rather than
		// an error, which is what makes asking twice harmless: the layers
		// below round up, so a chain that reached its size holds more than
		// was asked of it.
		//
		// Asking for exactly what is held is not nothing: a chain left uneven
		// by a resize that stopped part way holds its size at the head and
		// not below, and asking again is how it is finished.
		//
		// Nor is asking less of a head spanning what is below it, as a
		// filesystem: it is counted as its device, so a filesystem left
		// short of its device, as by a device grown by hand, holds more
		// than any size below the device asks, and is grown onto it, as
		// asking for the size of the device does.
		//
		// A filesystem alone: a volume group or a drbd device grown onto
		// what is below it takes the whole of it, which asking for less
		// than it holds is not asking for.
		if !isFilesystem(head.r) {
			return plan, nil
		}
		if spanner, ok := head.r.(resource.ResizeSpansBelow); !ok || !spanner.ResizeSpansBelow() {
			return plan, nil
		}
		plan.Stages = stages[0] + 1
		levelSteps := make([][]ResizeStep, 0, len(chain))
		for levelIndex, level := range chain {
			levelSteps = append(levelSteps, notAskedSteps(ctx, level, stages[levelIndex], "holds the size asked"))
		}
		reverse(levelSteps)
		for _, steps := range levelSteps {
			plan.Steps = append(plan.Steps, steps...)
		}
		unskipSpansBelow(&plan)
		return plan, nil
	}
	return buildResizePlanTo(ctx, chain, to, home, stages, 0)
}

// isFilesystem says r is a filesystem resource.
func isFilesystem(r resource.Driver) bool {
	return r.DriverID().Group == driver.GroupFS
}

// buildResizePlanTo works out what growing the head of a chain to the size to
// would do, from the top down, and changes nothing. stages says which stage
// each level is grown in.
//
// bottom, when not zero, is the size the bottom is asked, the size a volume
// is given: the bottom is asked it, not the smallest size handing the head
// its size, which the links above, rounding off what they hand out, would
// leave short of it. It is asked it even when a level above holds what is
// needed, for the configuration recording the bottom to say the size the
// bottom holds.
func buildResizePlanTo(ctx context.Context, chain []resizeLevel, to int64, home naming.Path, stages []int, bottom int64) (ResizePlan, error) {
	var plan ResizePlan
	head := chain[0][0]

	// The chain is indexed from the head, which is in the last stage.
	plan.Stages = stages[0] + 1

	last := len(chain) - 1
	levelSteps := make([][]ResizeStep, 0, len(chain))
	for levelIndex, level := range chain {
		if bottom > 0 && levelIndex > 0 && levelIndex == last && to < bottom {
			to = bottom
		}
		if levelIndex > 0 && levelHolds(ctx, level, to) {
			// A level holding what the level above asks of it is not asked
			// anything, and neither is what is below it: a logical volume
			// growing into the free space of its volume group asks nothing
			// of the array under the group, which may not be able to grow.
			//
			// The head is always asked: asking a chain for the size its head
			// holds is how a chain a resize left uneven is finished.
			levelSteps = append(levelSteps, notAskedSteps(ctx, level, stages[levelIndex], "already holds what the link above needs"))
			for i, rest := range chain[levelIndex+1:] {
				restIndex := levelIndex + 1 + i
				if bottom > 0 && restIndex == last {
					steps, _, err := askResizeLevel(ctx, rest, restIndex, bottom, stages[restIndex], head, home)
					if err != nil {
						return plan, err
					}
					levelSteps = append(levelSteps, steps)
					continue
				}
				levelSteps = append(levelSteps, notAskedSteps(ctx, rest, stages[restIndex], "not asked: a link above holds what is needed"))
			}
			break
		}
		steps, needBelow, err := askResizeLevel(ctx, level, levelIndex, to, stages[levelIndex], head, home)
		if err != nil {
			return plan, err
		}
		levelSteps = append(levelSteps, steps)
		to = needBelow
	}

	// A chain grows from the bottom up: the space has to exist before
	// anything is stretched onto it.
	//
	// The levels are what reverses, not the steps: the resources of one level
	// rest on nothing of each other, so their order among themselves is the
	// order they were found in either way.
	reverse(levelSteps)
	for _, steps := range levelSteps {
		plan.Steps = append(plan.Steps, steps...)
	}
	unskipSpansBelow(&plan)
	return plan, nil
}

// askResizeLevel works out the steps growing each resource of a level to the
// size to, and the size the level below has to hold for them, the most any
// of them asks. A level below the head holding the size is not asked how it
// would grow.
func askResizeLevel(ctx context.Context, level resizeLevel, levelIndex int, to int64, stage int, head resizeLink, home naming.Path) ([]ResizeStep, int64, error) {
	// Every resource of a level is asked for the same size, and the level
	// below has to satisfy the most demanding of them.
	var needBelow int64
	var steps []ResizeStep
	for _, link := range level {
		sizer, ok := link.r.(resource.Sizer)
		if !ok {
			return nil, 0, resizeRefusal(link, head, home, "reports no size")
		}
		linkFrom, err := sizer.CurrentSize(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: size: %w", resizeLinkName(link, home), err)
		}
		if levelIndex > 0 && linkFrom >= to {
			// A link holding the size is not asked how it would grow:
			// it does not, and one that cannot is no reason to refuse.
			steps = append(steps, ResizeStep{
				Path:    link.path.String(),
				RID:     link.r.RID(),
				Driver:  driverOf(link.r),
				From:    linkFrom,
				To:      linkFrom,
				Stage:   stage,
				Skip:    true,
				Comment: "already holds what the link above needs",
				r:       link.r,
				owner:   link.owner,
			})
			continue
		}
		resizer, ok := link.r.(resource.Resizer)
		if !ok {
			return nil, 0, resizeRefusal(link, head, home, "cannot resize")
		}
		linkNeedBelow, err := resizer.ResizePlan(ctx, to)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", resizeLinkName(link, home), err)
		}
		if err := checkUnmanagedBelow(ctx, link, linkNeedBelow); err != nil {
			return nil, 0, fmt.Errorf("%s: %w", resizeLinkName(link, home), err)
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
			Stage:  stage,
			r:      link.r,
			owner:  link.owner,
		}

		// A link below only has to be large enough. One that already is
		// is left alone: taking the space back is work nobody asked for.
		// This is also what heals a chain left uneven by a resize that
		// failed part way.
		if linkFrom >= to {
			step.Skip = true
			step.To = linkFrom
			step.Comment = "already holds what the link above needs"
		} else if linkNeedBelow != to {
			step.Comment = fmt.Sprintf("asks %s of the link below", sizeconv.BSizeCompact(float64(linkNeedBelow)))
		}
		steps = append(steps, step)
	}
	return steps, needBelow, nil
}

// resolveBottomSize is the size the bottom of a chain is asked to hold. An
// amount to add is added to what the bottom holds, which is the smallest of
// what its links hold: a level is grown as one.
func resolveBottomSize(ctx context.Context, chain []resizeLevel, change sizeconv.Change, home naming.Path) (int64, error) {
	head := chain[0][0]
	if !change.IsRelative {
		if change.Value <= 0 {
			return 0, fmt.Errorf("%s: %s leaves nothing", resizeLinkName(head, home), change)
		}
		return change.Value, nil
	}
	var from int64 = -1
	for _, link := range chain[len(chain)-1] {
		sizer, ok := link.r.(resource.Sizer)
		if !ok {
			return 0, resizeRefusal(link, head, home, "reports no size")
		}
		size, err := sizer.CurrentSize(ctx)
		if err != nil {
			return 0, fmt.Errorf("%s: size: %w", resizeLinkName(link, home), err)
		}
		if from < 0 || size < from {
			from = size
		}
	}
	to := change.Resolve(from)
	if to <= 0 {
		return 0, fmt.Errorf("%s: %s of %s leaves nothing", resizeLinkName(head, home), change, sizeconv.BSizeCompact(float64(from)))
	}
	return to, nil
}

// headSizeFromBottom works out the size the head of a chain has to reach for
// its bottom to hold bottom. See chainHandsUp.
func headSizeFromBottom(ctx context.Context, chain []resizeLevel, bottom int64, home naming.Path) (int64, error) {
	handsUp, err := chainHandsUp(ctx, chain, home)
	if err != nil {
		return 0, err
	}
	return handsUp(bottom), nil
}

// chainHandsUp answers the function working out the size the head of a chain
// reaches from the size its bottom holds: each level hands out to the level
// above what its links hand out of what the level below holds, the least of
// them, a level being grown as one. A link that cannot say hands out what is
// below it.
//
// What the links rest their answer on is measured once, here, so the function
// can be asked of many sizes, as the search for the smallest size growing a
// chain does.
func chainHandsUp(ctx context.Context, chain []resizeLevel, home naming.Path) (func(bottom int64) int64, error) {
	levels := make([][]func(int64) int64, len(chain))
	for i := 0; i < len(chain)-1; i++ {
		for _, link := range chain[i] {
			f := resource.HandsOutWhatIsBelow
			if h, ok := link.r.(resource.ResizeHandsOuter); ok {
				g, err := h.ResizeHandsOut(ctx)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", resizeLinkName(link, home), err)
				}
				f = g
			}
			levels[i] = append(levels[i], f)
		}
	}
	return func(bottom int64) int64 {
		size := bottom
		for i := len(chain) - 2; i >= 0; i-- {
			var levelSize int64 = -1
			for _, f := range levels[i] {
				if out := f(size); levelSize < 0 || out < levelSize {
					levelSize = out
				}
			}
			size = levelSize
		}
		return size
	}, nil
}

// bottomUnit is what the bottom of a chain grows by, the largest of what its
// links grow by: a logical volume grows by extents, a loop file by sectors.
func bottomUnit(ctx context.Context, chain []resizeLevel) int64 {
	unit := int64(512)
	for _, link := range chain[len(chain)-1] {
		i, ok := link.r.(resource.ResizeUniter)
		if !ok {
			continue
		}
		if u, err := i.ResizeUnit(ctx); err == nil && u > unit {
			unit = u
		}
	}
	return unit
}

// smallestGrowingBottom is the smallest size of the bottom, a whole number of
// unit more than bottom, for which the head reaches more than it holds, and
// false when no size does. It searches up from bottom, doubling the step
// until a size grows the head, then halves its way back down to the smallest
// one: what a chain hands up never shrinks as its bottom grows.
func smallestGrowingBottom(handsUp func(int64) int64, bottom, head, unit int64) (int64, bool) {
	if unit <= 0 {
		unit = 512
	}
	lo := sizeconv.RoundUp(bottom, unit)
	if lo > bottom && handsUp(lo) > head {
		return lo, true
	}
	step := unit
	hi := lo + step
	for handsUp(hi) <= head {
		if step > 1<<52 {
			return 0, false
		}
		step *= 2
		hi = lo + step
	}
	for hi-lo > unit {
		mid := lo + (hi-lo)/unit/2*unit
		if handsUp(mid) > head {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi, true
}

// ResizeGrowsNothingError refuses a size asked of a volume, more than the size
// it is configured to hold, that grows nothing: the links of its chain keep a
// cut of what they rest on and hand out whole units, so a bottom grown by
// less than that hands up no more than the head already holds. The size would
// be written to the configuration and claimed of the pool, and nothing would
// hold it.
type ResizeGrowsNothingError struct {
	// Name is the volume, or the link the size of the chain was worked out
	// up to.
	Name string

	// Asked is the size asked of the bottom, and Head what the head holds,
	// which is no less than what Asked leaves it.
	Asked, Head int64

	// Smallest is the smallest size of the bottom that grows the chain, and
	// 0 when none does.
	Smallest int64
}

func (e *ResizeGrowsNothingError) Error() string {
	s := fmt.Sprintf("%s: a size of %s grows nothing: the links of its chain keep a cut of what they rest on and hand out whole units, and what is left is no more than the %s its head holds",
		e.Name, sizeconv.ExactBSizeCompact(float64(e.Asked)), sizeconv.ExactBSizeCompact(float64(e.Head)))
	if e.Smallest > 0 {
		s += fmt.Sprintf(". The smallest size growing it is %s", sizeconv.ExactBSizeCompact(float64(e.Smallest)))
	}
	return s
}

// growsNothingError is the refusal of a size asked of the bottom of a chain
// whose head already holds what it leaves it.
func growsNothingError(ctx context.Context, chain []resizeLevel, handsUp func(int64) int64, name string, bottom, head int64) error {
	e := &ResizeGrowsNothingError{Name: name, Asked: bottom, Head: head}
	if smallest, ok := smallestGrowingBottom(handsUp, bottom, head, bottomUnit(ctx, chain)); ok {
		e.Smallest = smallest
	}
	return e
}

// CheckResizeGrows refuses a size asked of the bottom of the chain of rid that
// grows nothing, naming the smallest size that does. It changes nothing.
//
// The chain is walked from rid, and from the replicated resource when rid
// cannot be read here: a node not holding the object up cannot read the
// filesystem of its head, and a filesystem taking up the whole of the
// replicated device is grown to what it hands up. A chain readable neither way
// cannot be checked here, which is an error other than the refusal.
func (t *actor) CheckResizeGrows(ctx context.Context, rid string, bottom int64) error {
	chain, err := t.resizeChainOf(ctx, rid, map[string]bool{})
	if err != nil {
		return err
	}
	from, err := chainHeadSize(ctx, chain)
	if err != nil {
		r := t.replicatedResizeResource(ctx)
		if r == nil {
			return err
		}
		if chain, err = t.resizeChain(ctx, r, map[string]bool{}); err != nil {
			return err
		}
		if from, err = chainHeadSize(ctx, chain); err != nil {
			return err
		}
	}
	handsUp, err := chainHandsUp(ctx, chain, t.path)
	if err != nil {
		return err
	}
	if handsUp(bottom) > from {
		return nil
	}
	return growsNothingError(ctx, chain, handsUp, t.path.String(), bottom, from)
}

// chainHeadSize is what the head of a chain holds.
func chainHeadSize(ctx context.Context, chain []resizeLevel) (int64, error) {
	if len(chain) == 0 || len(chain[0]) == 0 {
		return 0, fmt.Errorf("nothing to resize")
	}
	sizer, ok := chain[0][0].r.(resource.Sizer)
	if !ok {
		return 0, fmt.Errorf("%s reports no size", chain[0][0].r.RID())
	}
	return sizer.CurrentSize(ctx)
}

// levelHolds says every link of a level holds the size asked of it.
func levelHolds(ctx context.Context, level resizeLevel, to int64) bool {
	for _, link := range level {
		sizer, ok := link.r.(resource.Sizer)
		if !ok {
			return false
		}
		from, err := sizer.CurrentSize(ctx)
		if err != nil || from < to {
			return false
		}
	}
	return true
}

// notAskedSteps lists the links of a level a resize asks nothing of, a level
// above holding what is needed, so a plan shows the whole chain.
func notAskedSteps(ctx context.Context, level resizeLevel, stage int, comment string) []ResizeStep {
	steps := make([]ResizeStep, 0, len(level))
	for _, link := range level {
		var from int64
		if sizer, ok := link.r.(resource.Sizer); ok {
			from, _ = sizer.CurrentSize(ctx)
		}
		steps = append(steps, ResizeStep{
			Path:    link.path.String(),
			RID:     link.r.RID(),
			Driver:  driverOf(link.r),
			From:    from,
			To:      from,
			Stage:   stage,
			Skip:    true,
			Comment: comment,
			r:       link.r,
			owner:   link.owner,
		})
	}
	return steps
}

// unmanagedDevices returns the devices a link rests on that no resource of
// its object handles: nothing a resize walks grows them.
func unmanagedDevices(ctx context.Context, link resizeLink) []string {
	if link.owner == nil {
		return nil
	}
	if namer, ok := link.r.(resource.ResizeRestsOn); ok {
		if link.owner.resizeProvider(ctx, namer.ResizeRestsOn(ctx)) != nil {
			return nil
		}
	}
	sub, ok := link.r.(resource.SubDeviceser)
	if !ok {
		return nil
	}
	l := make([]string, 0)
	for _, dev := range sub.SubDevices(ctx) {
		if below, err := link.owner.ResourceHandlingDevice(ctx, dev); err == nil && below == nil {
			l = append(l, dev.Path())
		}
	}
	return l
}

// deviceSize reads the size of a device, and is replaced by the tests.
var deviceSize = func(p string) (int64, error) {
	return device.New(p).Size()
}

// checkUnmanagedBelow refuses a link asking of a device no resource handles
// more than it holds: nothing grows it, so the resize would fail once the
// links above are grown, as mdadm does on members that are disks.
//
// What a link asks below is worked out by formulas rounding up, to chunks
// and extents, or adding the overhead of a member read from another, so a
// device a little short of it may still do: the resize tool decides then.
func checkUnmanagedBelow(ctx context.Context, link resizeLink, need int64) error {
	return refuseShortDevices(need, unmanagedDevices(ctx, link))
}

// refuseShortDevices refuses a need larger than what one of the devices
// holds, beyond the tolerance of the formulas.
func refuseShortDevices(need int64, paths []string) error {
	for _, p := range paths {
		size, err := deviceSize(p)
		if err != nil || size <= 0 {
			continue
		}
		if need > size+unmanagedTolerance(size) {
			return fmt.Errorf("it asks %s of %s, which holds %s, and nothing below it can grow", sizeconv.BSizeCompact(float64(need)), p, sizeconv.BSizeCompact(float64(size)))
		}
	}
	return nil
}

// unmanagedTolerance is how much more than a device holds a link may ask of
// it before the plan refuses: 1% of the device, 1MiB at the least.
func unmanagedTolerance(size int64) int64 {
	t := size / 100
	if t < 1024*1024 {
		t = 1024 * 1024
	}
	return t
}

// unskipSpansBelow puts back the steps that were skipped for holding the size
// asked of them, but are grown onto what is below them rather than to a size.
//
// A filesystem is the size of its device, and a chain grows from the bottom
// up, so by the time the filesystem is reached the device already holds the
// new size and the two are equal. Comparing them says there is nothing to do,
// of a filesystem that has not been grown at all. What says otherwise is that
// something below it grew.
func unskipSpansBelow(plan *ResizePlan) {
	grown := false
	for i := range plan.Steps {
		if !plan.Steps[i].Skip {
			grown = true
			continue
		}
		spanner, ok := plan.Steps[i].r.(resource.ResizeSpansBelow)
		if !ok || !spanner.ResizeSpansBelow() {
			continue
		}
		// The last step of a grow is the resource the resize was asked of.
		// Asking it to take up its device is what repairs a device grown by
		// hand and left with a filesystem short of it, which no size here can
		// report: the filesystem is counted as the device either way. Growing
		// one that already spans its device changes nothing.
		if !grown && i != len(plan.Steps)-1 {
			continue
		}
		plan.Steps[i].Skip = false
		plan.Steps[i].To = plan.Steps[i].From
		plan.Steps[i].Comment = "grown onto what is below it"
	}
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
		if err := step.recordSize(ctx, plan.Bottom); err != nil {
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
//
// One keyword is written alongside the resource rather than instead of it:
// the size of a volume the resource was sized from, which is the claim the
// cluster rations its namespace by. See claimSizeKey.
//
// limit, when not 0, is the size asked of the bottom of the chain, and
// DEFAULT.size is never recorded more than it. That size is the claim the
// cluster checked, and what a link rounds up to on the way down, a loop file
// to whole sectors, is not the claim outgrown: recording it would take a
// little more of the pool at every resize than anyone asked for.
func (t ResizeStep) recordSize(ctx context.Context, limit int64) error {
	if t.owner == nil {
		return nil
	}
	k := key.T{Section: t.RID, Option: "size"}
	if !t.owner.config.HasKey(k) {
		return nil
	}
	was := t.owner.config.Get(k)
	keys := []key.T{k}

	// A reference says the value lives in another keyword, so that is the one
	// to record in. A pool-served volume points the size of its resources at
	// DEFAULT.size, which is also the size the pool counts the volume as
	// claiming, so leaving it alone lets a resize grow the storage without the
	// cluster ever hearing that the claim grew with it.
	if ref, ok := referencedKey(was); ok {
		keys = []key.T{ref}
		was = t.owner.config.Get(ref)
	} else if claim, ok := t.claimSizeKey(k); ok {
		keys = append(keys, claim)
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
	if limit > 0 && reached > limit && recordsClaim(keys) {
		t.owner.log.Infof("%s: reached %s, more than the %s asked: %s is recorded", t.RID,
			sizeconv.ExactBSizeCompact(float64(reached)),
			sizeconv.ExactBSizeCompact(float64(limit)),
			sizeconv.ExactBSizeCompact(float64(limit)))
		reached = limit
	}
	value := sizeconv.ExactBSizeCompact(float64(reached))
	ops := make([]keyop.T, 0, len(keys))
	for _, k := range keys {
		was := t.owner.config.Get(k)
		if value == was {
			continue
		}
		t.owner.log.Infof("record %s %s -> %s", k, was, value)
		ops = append(ops, keyop.T{Key: k, Op: keyop.Set, Value: value})
	}
	if len(ops) == 0 {
		return nil
	}
	return t.owner.config.Set(ops...)
}

// recordsClaim says whether DEFAULT.size, the size a volume claims of its
// pool, is among the keywords a size is recorded in.
func recordsClaim(keys []key.T) bool {
	for _, k := range keys {
		if k.Section == "DEFAULT" && k.Option == "size" {
			return true
		}
	}
	return false
}

// claimSizeKey is the size of the object, when this resource is the one a
// pool sized from the claim the namespace holds on it.
//
// A pool writes the size of the resource it serves as a reference to
// DEFAULT.size, so that recording follows the reference and the storage and
// the claim the cluster rations move together. A configuration written before
// that holds the same number twice instead, and recording the resource alone
// leaves the cluster counting a claim the storage has outgrown: a volume
// grown from 512mi to 612mi still weighs 512mi on the claim of its namespace,
// and on every capacity decision read from it.
//
// The two are recorded together only while they agree, which is how the pool
// left them. A resource holding something else was sized from something else,
// and the claim of the volume is not its to carry: the logical volume of a
// drbd pool volume takes what its group has, and what the pool was asked for
// is what the loop file below it holds.
func (t ResizeStep) claimSizeKey(k key.T) (key.T, bool) {
	claim := key.T{Section: "DEFAULT", Option: "size"}
	if t.owner.path.Kind != naming.KindVol {
		return claim, false
	}
	if t.owner.config.GetString(key.T{Section: "DEFAULT", Option: "pool"}) == "" {
		// Served by no pool, so claimed from nothing.
		return claim, false
	}
	if !isRecordableSize(t.owner.config.Get(claim)) {
		return claim, false
	}
	resourceSize := t.owner.config.GetSize(k)
	objectSize := t.owner.config.GetSize(claim)
	if resourceSize == nil || objectSize == nil || *resourceSize != *objectSize {
		return claim, false
	}
	return claim, true
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
