package omcmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
)

type (
	CmdObjectConfigValidate struct {
		OptsGlobal
		commoncmd.OptsLock
	}
)

func (t *CmdObjectConfigValidate) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	alerts := make(xconfig.Alerts, 0)
	alertsQ := make(chan xconfig.Alerts)
	done := make(chan bool)
	go func() {
		for {
			select {
			case moreAlerts := <-alertsQ:
				alerts = append(alerts, moreAlerts...)
			case <-done:
				return
			}
		}
	}()

	var wg sync.WaitGroup

	err := objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			wg.Add(1)
			defer wg.Done()
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			c, ok := o.(object.Configurer)
			if !ok {
				return nil, fmt.Errorf("%s is not a configurer", o)
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			if moreAlerts, err := c.ValidateConfig(ctx); err != nil {
				return nil, err
			} else {
				// The checks the drivers make when the object loads
				// its resources, on this node.
				if v, ok := o.(interface{ ValidateResources() xconfig.Alerts }); ok {
					moreAlerts = append(moreAlerts, v.ValidateResources()...)
				}
				alertsQ <- moreAlerts
			}
			return nil, nil
		}),
	).Do()

	if err != nil {
		return err
	}

	wg.Wait()
	done <- true

	if err := (output.Renderer{
		DefaultOutput: "tab=LEVEL:icon,PATH:path,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          alerts,
		Colorize:      rawconfig.Colorize,
	}.Print()); err != nil {
		return err
	}
	return errorsOf(alerts)
}

// errorsOf returns an error when alerts has errors, so the validation exits
// non-zero for a configuration that is not good.
func errorsOf(alerts xconfig.Alerts) error {
	if n := len(alerts.Errors()); n > 0 {
		return fmt.Errorf("the configuration has %d error(s)", n)
	}
	return nil
}
