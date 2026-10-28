package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/drivercatalog"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNodeDrivers struct {
		Color  string
		Output string
		Sort   string
	}
)

func (t *CmdNodeDrivers) Run() error {
	return output.Renderer{
		DefaultOutput: commoncmd.NodeDriversDefaultOutput,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          drivercatalog.APIList(hostname.Hostname()),
		Colorize:      rawconfig.Colorize,
	}.Print()
}
