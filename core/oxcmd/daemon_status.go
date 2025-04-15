package oxcmd

import (
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/monitor"
)

type (
	CmdClusterStatus struct {
		OptsGlobal
		Watch    bool
		Sections string
	}
)

func (t *CmdClusterStatus) Run() error {
	m := monitor.New()
	m.SetColor(t.Color)
	m.SetFormat(t.Output)
	m.SetSectionsFromExpression(t.Sections)

	cli, err := client.New()
	if err != nil {
		return err
	}
	if t.Watch {
		statusGetter := cli.NewGetClusterStatus().SetSelector(t.ObjectSelector)
		evReader, err := cli.NewGetEvents().SetSelector(t.ObjectSelector).GetReader()
		if err != nil {
			return err
		}
		return m.DoWatch(statusGetter, evReader, os.Stdout)
	} else {
		getter := cli.NewGetClusterStatus().SetSelector(t.ObjectSelector)
		return m.Do(getter, os.Stdout)
	}
}
