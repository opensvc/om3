package object

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// ResizeStep is one link of the chain a resize walks, and the size that
	// link is asked to reach.
	ResizeStep struct {
		RID     string `json:"rid"`
		Driver  string `json:"driver"`
		From    int64  `json:"from"`
		To      int64  `json:"to"`
		Comment string `json:"comment,omitempty"`
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
	}
)

func (t ResizeStep) String() string {
	s := fmt.Sprintf("%-16s %-18s %10s -> %-10s",
		t.RID, t.Driver,
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
	lines := make([]string, 0, len(t.Steps)+1)
	lines = append(lines, fmt.Sprintf("%s, in this order:", direction))
	for _, step := range t.Steps {
		lines = append(lines, "  "+step.String())
	}
	return strings.Join(lines, "\n")
}

// resizeChain returns the resources a resize of rid has to change, from the
// named one down to the deepest one it rests on.
//
// A link is found by asking the resource what devices it sits on, and the
// object which resource exposes each of them. The walk stops where no resource
// of the object exposes the device: below that the object owns nothing.
func (t *actor) resizeChain(ctx context.Context, r resource.Driver) ([]resource.Driver, error) {
	chain := []resource.Driver{r}
	seen := map[string]bool{r.RID(): true}
	for {
		sub, ok := chain[len(chain)-1].(resource.SubDeviceser)
		if !ok {
			return chain, nil
		}
		var next resource.Driver
		for _, dev := range sub.SubDevices(ctx) {
			below, err := t.ResourceHandlingDevice(ctx, dev)
			if err != nil {
				return nil, err
			}
			if below == nil || seen[below.RID()] {
				continue
			}
			if next != nil && next.RID() != below.RID() {
				// Several resources of the object below this one. Which of
				// them a size belongs to is not something this can decide.
				return nil, fmt.Errorf("%s rests on %s and %s: a resize of a chain that forks is not supported",
					chain[len(chain)-1].RID(), next.RID(), below.RID())
			}
			next = below
		}
		if next == nil {
			return chain, nil
		}
		seen[next.RID()] = true
		chain = append(chain, next)
	}
}

// ResizePlan works out what resizing rid to the given size would do, and
// changes nothing.
//
// Every link is asked, so a chain holding one link that cannot do it is
// refused whole. A link answers the size it needs from the link below, which
// is not always the size it was asked for: a raid6 md holding n devices needs
// to(n-2) from each.
func (t *actor) ResizePlan(ctx context.Context, rid string, change sizeconv.Change) (ResizePlan, error) {
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
	r := selected[0]
	chain, err := t.resizeChain(ctx, r)
	if err != nil {
		return plan, err
	}
	return BuildResizePlan(ctx, chain, change)
}

// BuildResizePlan works out what resizing a chain would do, and changes
// nothing. The chain runs from the resource the size was asked of down to the
// deepest one it rests on.
func BuildResizePlan(ctx context.Context, chain []resource.Driver, change sizeconv.Change) (ResizePlan, error) {
	var plan ResizePlan
	if len(chain) == 0 {
		return plan, fmt.Errorf("nothing to resize")
	}

	// The size asked for is of the resource named, so the direction is read
	// from that one. The links below follow it.
	head, ok := chain[0].(resource.Sizer)
	if !ok {
		return plan, fmt.Errorf("%s: driver %s does not report a size", chain[0].RID(), driverOf(chain[0]))
	}
	from, err := head.CurrentSize(ctx)
	if err != nil {
		return plan, fmt.Errorf("%s: size: %w", chain[0].RID(), err)
	}
	to := change.Resolve(from)
	if to <= 0 {
		return plan, fmt.Errorf("%s: %s of %s leaves nothing", chain[0].RID(), change, sizeconv.BSizeCompact(float64(from)))
	}
	plan.IsShrink = to < from

	for _, link := range chain {
		sizer, ok := link.(resource.Sizer)
		if !ok {
			return plan, fmt.Errorf("%s: driver %s does not report a size, so the chain cannot be resized",
				link.RID(), driverOf(link))
		}
		resizer, ok := link.(resource.Resizer)
		if !ok {
			return plan, fmt.Errorf("%s: driver %s cannot be resized, so the chain cannot be",
				link.RID(), driverOf(link))
		}
		linkFrom, err := sizer.CurrentSize(ctx)
		if err != nil {
			return plan, fmt.Errorf("%s: size: %w", link.RID(), err)
		}
		needBelow, err := resizer.ResizePlan(ctx, to)
		if err != nil {
			return plan, fmt.Errorf("%s: %w", link.RID(), err)
		}
		step := ResizeStep{
			RID:    link.RID(),
			Driver: driverOf(link),
			From:   linkFrom,
			To:     to,
		}
		if needBelow != to {
			step.Comment = fmt.Sprintf("asks %s of the link below", sizeconv.BSizeCompact(float64(needBelow)))
		}
		plan.Steps = append(plan.Steps, step)
		to = needBelow
	}

	// A chain grows from the bottom up: the space has to exist before
	// anything is stretched onto it. It shrinks from the top down: a
	// filesystem has to give the space back before the device under it is
	// taken away, or what is still mounted is larger than what holds it.
	if !plan.IsShrink {
		reverse(plan.Steps)
	}
	return plan, nil
}

// Resize changes the size of rid and of everything it rests on.
func (t *actor) Resize(ctx context.Context, rid string, change sizeconv.Change) error {
	plan, err := t.ResizePlan(ctx, rid, change)
	if err != nil {
		return err
	}
	for _, step := range plan.Steps {
		r := t.ResourceByID(step.RID)
		resizer, ok := r.(resource.Resizer)
		if !ok {
			// The plan said otherwise a moment ago.
			return fmt.Errorf("%s: cannot be resized", step.RID)
		}
		t.log.Infof("resize %s from %s to %s", step.RID,
			sizeconv.BSizeCompact(float64(step.From)),
			sizeconv.BSizeCompact(float64(step.To)))
		if err := resizer.Resize(ctx, step.To); err != nil {
			return fmt.Errorf("%s: %w", step.RID, err)
		}
	}
	return nil
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

func reverse(l []ResizeStep) {
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
}
