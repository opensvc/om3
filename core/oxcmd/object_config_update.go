package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectConfigUpdate struct {
		OptsGlobal
		commoncmd.OptsLock
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

	if !t.Wait {
		var cancelUpdate context.CancelFunc
		ctx, cancelUpdate = context.WithTimeout(ctx, time.Second*5)
		defer cancelUpdate()
	}

	errC := make(chan error)
	doneC := make(chan string)
	todo := len(paths)

	prefix := ""
	noPrefix := len(paths) == 1

	for _, path := range paths {
		go func(p naming.Path) {
			defer func() { doneC <- p.String() }()
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
				errC <- err
				return
			}
			if !noPrefix {
				prefix = p.String() + ": "
			}
			if changed {
				fmt.Printf("%scommitted\n", prefix)
			} else {
				fmt.Printf("%sunchanged\n", prefix)
			}
		}(path)
	}

	var (
		errs error
		done int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case <-doneC:
			done++
			if done == todo {
				return errs
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			return errs
		}
	}
}
