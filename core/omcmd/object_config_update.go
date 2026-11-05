package omcmd

import (
	"context"
	"fmt"
	"os"
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
		Force  bool

		// DryRun has the daemon run every check of the update, and write
		// nothing.
		DryRun bool
	}
)

func (t *CmdObjectConfigUpdate) Run(kind string) error {
	if len(t.Delete) == 0 && len(t.Set) == 0 && len(t.Unset) == 0 {
		fmt.Println("no changes requested")
		return nil
	}
	if t.DryRun && t.Wait {
		return fmt.Errorf("--dry-run and --wait are mutually exclusive: a dry run writes nothing to wait for")
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Local {
		if t.DryRun {
			return fmt.Errorf("--dry-run and --local are mutually exclusive: the checks of an update are the daemon's, and --local writes without it")
		}
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
		if t.Force {
			params.Force = &t.Force
		}
		if t.DryRun {
			params.DryRun = &t.DryRun
		}
		if t.Wait {
			wait := t.Time.String()
			params.Wait = &wait
		}
		changed, warnings, err := commoncmd.PatchObjectConfig(ctx, c, p, params)
		if err != nil {
			return err
		}
		if !noPrefix {
			prefix = p.String() + ": "
		}
		if changed && t.DryRun {
			fmt.Printf("%swould be committed\n", prefix)
		} else if changed {
			fmt.Printf("%scommitted\n", prefix)
		} else {
			fmt.Printf("%sunchanged\n", prefix)
		}
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "%swarning: %s\n", prefix, w)
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
			unsets, sets := key.ParseStrings(t.Unset), keyop.ParseOps(t.Set)
			if !t.Force {
				if err := object.CheckLocalUpdateStopToChange(p, t.Delete, unsets, sets); err != nil {
					return nil, err
				}
			}
			return nil, o.Update(ctx, t.Delete, unsets, sets)
		}),
	).Do()
}
