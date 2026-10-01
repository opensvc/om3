package omcmd

import (
	"context"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdNodePushSwitches struct {
		OptsGlobal
		Local                       bool
		Switch                      string
		IgnoreNoCollectorConfigured bool
	}
)

// Run pushes the switches and renders a row per switch, the ones that failed
// included: a switch out of reach does not hide the ones that were pushed.
func (t *CmdNodePushSwitches) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	l, err := n.PushSwitches(context.Background(), t.Switch)
	if err != nil && t.IgnoreNoCollectorConfigured && isNoCollectorError(err) {
		return nil
	}
	if len(l) > 0 {
		if renderErr := (output.Renderer{
			DefaultOutput: "tab=NAME:name,TYPE:type,COMMANDS:commands,VIA:via,ERROR:error",
			Output:        t.Output,
			Sort:          t.Sort,
			Color:         t.Color,
			Data:          l,
			Colorize:      rawconfig.Colorize,
		}).Print(); renderErr != nil {
			return renderErr
		}
	}
	return err
}
