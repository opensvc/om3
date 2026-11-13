package omcmd

import (
	"sync"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xconfig"
)

type (
	CmdNodeConfigValidate struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run validates the configuration of this node, or with a node selector,
// which only the hidden "node validate config" offers, the configurations of
// the nodes selected, each on its node through the api.
func (t *CmdNodeConfigValidate) Run() error {
	if t.NodeSelector != "" {
		return commoncmd.NodeConfigValidate(t.NodeSelector, t.OptsGlobal.OptsGlobal)
	}
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

	err := nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			if moreAlerts, err := n.ValidateConfig(); err != nil {
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

	if err := (output.Renderer{
		DefaultOutput: "tab=LEVEL:icon,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment",
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
