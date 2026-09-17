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
		Size          string
		DryRun        bool
		Stage         int
		SkipHeadStage bool
		Force         bool
	}

	instanceResizer interface {
		ConfiguredSize() (int64, error)
		HeadRID(context.Context) (string, error)
		ResizePlan(context.Context, string, sizeconv.Change, object.ResizeOptions) (object.ResizePlan, error)
		Resize(context.Context, string, sizeconv.Change, object.ResizeOptions) error
		ResizePlanStage(context.Context, string, int64, int, object.ResizeOptions) (object.ResizePlan, error)
		ResizeStage(context.Context, string, int64, int, object.ResizeOptions) error
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
	opts := object.ResizeOptions{Force: t.Force, SkipHeadStage: t.SkipHeadStage}

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

	rid, err := i.HeadRID(ctx)
	if err != nil {
		return err
	}

	if t.Stage >= 0 {
		if change.IsRelative {
			// Resolving one needs the head, and a node running an early stage
			// may not hold the object up, so it cannot read it.
			return fmt.Errorf("--stage needs a size to reach, not an amount to add or remove")
		}
		plan, err := i.ResizePlanStage(ctx, rid, change.Value, t.Stage, opts)
		if err != nil {
			return err
		}
		if t.DryRun {
			return printResizePlan(plan, t.Output, t.Sort, t.Color)
		}
		return i.ResizeStage(ctx, rid, change.Value, t.Stage, opts)
	}
	plan, err := i.ResizePlan(ctx, rid, change, opts)
	if err != nil {
		return err
	}
	if t.DryRun {
		return printResizePlan(plan, t.Output, t.Sort, t.Color)
	}
	return i.Resize(ctx, rid, change, opts)
}
