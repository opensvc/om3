package omcmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/xconfig"
)

type (
	CmdObjectValidateConfig struct {
		OptsGlobal
		commoncmd.OptsLock
	}
)

func (t *CmdObjectValidateConfig) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
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
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
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

	output.Renderer{
		DefaultOutput: "tab=LEVEL:icon,PATH:path,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment",
		Output:        t.Output,
		Color:         t.Color,
		Data:          alerts,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
