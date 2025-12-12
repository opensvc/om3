package omcmd

import (
	"sync"

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

func (t *CmdNodeConfigValidate) Run() error {
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
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
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

	output.Renderer{
		DefaultOutput: "tab=LEVEL:icon,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment",
		Output:        t.Output,
		Color:         t.Color,
		Data:          alerts,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
