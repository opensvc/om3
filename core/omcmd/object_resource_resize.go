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
	CmdObjectResourceResize struct {
		OptsGlobal
		RID    string
		Size   string
		DryRun bool
	}
)

func (t *CmdObjectResourceResize) Run(kind string) error {
	change, err := sizeconv.ParseChange(t.Size)
	if err != nil {
		return err
	}
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
	return t.one(paths[0], change)
}

func (t *CmdObjectResourceResize) one(p naming.Path, change sizeconv.Change) error {
	type resizer interface {
		ResizePlan(context.Context, string, sizeconv.Change) (object.ResizePlan, error)
		Resize(context.Context, string, sizeconv.Change) error
	}
	o, err := object.New(p)
	if err != nil {
		return err
	}
	i, ok := o.(resizer)
	if !ok {
		return fmt.Errorf("%s: a %s has no resource to resize", p, p.Kind)
	}
	ctx := context.Background()
	plan, err := i.ResizePlan(ctx, t.RID, change)
	if err != nil {
		return err
	}
	if t.DryRun {
		fmt.Println(plan.String())
		return nil
	}
	return i.Resize(ctx, t.RID, change)
}
