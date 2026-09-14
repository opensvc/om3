package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	CmdObjectInstanceResize struct {
		OptsGlobal
		Size            string
		DryRun          bool
		BelowReplicated bool
		GrowOnly        bool
		Force           bool
	}

	instanceResizer interface {
		ConfiguredSize() (int64, error)
		HeadRID(context.Context) (string, error)
		ResizePlan(context.Context, string, sizeconv.Change, object.ResizeOptions) (object.ResizePlan, error)
		Resize(context.Context, string, sizeconv.Change, object.ResizeOptions) error
		ResizePlanBelowReplicated(context.Context, int64, object.ResizeOptions) (object.ResizePlan, error)
		ResizeBelowReplicated(context.Context, int64, object.ResizeOptions) error
	}
)

func (t *CmdObjectInstanceResize) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	paths, err := objectselector.New(mergedSelector).Expand()
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no object matching %s", mergedSelector)
	}
	if len(paths) > 1 {
		return fmt.Errorf("%s matches %d objects: a resize is asked of one", mergedSelector, len(paths))
	}
	return t.one(paths[0])
}

func (t *CmdObjectInstanceResize) one(p naming.Path) error {
	o, err := object.New(p)
	if err != nil {
		return err
	}
	i, ok := o.(instanceResizer)
	if !ok {
		return fmt.Errorf("%s: a %s has no head resource to resize", p, p.Kind)
	}
	ctx := context.Background()
	opts := object.ResizeOptions{GrowOnly: t.GrowOnly, Force: t.Force}

	// No size asked for means the size the volume is configured to be, which
	// is the size a resize orchestration is converging every node to.
	change := sizeconv.Change{}
	if t.Size == "" {
		to, err := i.ConfiguredSize()
		if err != nil {
			return err
		}
		change.Value = to
	} else if change, err = sizeconv.ParseChange(t.Size); err != nil {
		return err
	}

	if t.BelowReplicated {
		if change.IsRelative {
			// Resolving one needs the head, and a node that does not hold the
			// object up cannot read it: that is why this phase exists.
			return fmt.Errorf("--below-replicated needs a size to reach, not an amount to add or remove")
		}
		plan, err := i.ResizePlanBelowReplicated(ctx, change.Value, opts)
		if err != nil {
			return err
		}
		if t.DryRun {
			fmt.Println(plan.String())
			return nil
		}
		return i.ResizeBelowReplicated(ctx, change.Value, opts)
	}

	rid, err := i.HeadRID(ctx)
	if err != nil {
		return err
	}
	plan, err := i.ResizePlan(ctx, rid, change, opts)
	if err != nil {
		return err
	}
	if t.DryRun {
		fmt.Println(plan.String())
		return nil
	}
	return i.Resize(ctx, rid, change, opts)
}
