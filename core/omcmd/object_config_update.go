package omcmd

import (
	"context"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	CmdObjectConfigUpdate struct {
		OptsGlobal
		commoncmd.OptsLock
		Local  bool
		Delete []string
		Set    []string
		Unset  []string
		Wait   bool
		Time   time.Duration
	}
)

func (t *CmdObjectConfigUpdate) Run(kind string) error {
	if len(t.Delete) == 0 && len(t.Set) == 0 && len(t.Unset) == 0 {
		fmt.Println("no changes requested")
		return nil
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Local {
		if t.Wait {
			return fmt.Errorf("--wait needs the daemon, which is what propagates a configuration, and --local writes without it")
		}
		return t.doObjectAction(mergedSelector)
	}
	c, ctx, cancel, err := commoncmd.ConfigWaitClient(t.Wait, t.Time)
	if err != nil {
		return err
	}
	defer cancel()
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	noPrefix := len(paths) == 1
	prefix := ""
	for _, p := range paths {
		params := api.PatchObjectConfigParams{}
		params.Set = &t.Set
		params.Unset = &t.Unset
		params.Delete = &t.Delete
		if t.Wait {
			wait := t.Time.String()
			params.Wait = &wait
		}
		changed, err := commoncmd.PatchObjectConfig(ctx, c, p, params)
		if err != nil {
			return err
		}
		if !noPrefix {
			prefix = p.String() + ": "
		}
		if changed {
			fmt.Printf("%scommitted\n", prefix)
		} else {
			fmt.Printf("%sunchanged\n", prefix)
		}
	}
	return nil
}

func (t *CmdObjectConfigUpdate) doObjectAction(mergedSelector string) error {
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithOutput(t.Output),
		objectaction.WithSort(t.Sort),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return nil, o.Update(ctx, t.Delete, key.ParseStrings(t.Unset), keyop.ParseOps(t.Set))
		}),
	).Do()
}
