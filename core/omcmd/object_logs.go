package omcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdObjectLogs struct {
		OptsGlobal
		commoncmd.OptsLogs
		NodeSelector string
	}
)

func (t *CmdObjectLogs) Run(kind string) error {
	render.SetColor(t.Color)
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "**")
	if t.Local {
		return t.local(mergedSelector)
	} else {
		return t.asCommonCmd().Remote(mergedSelector)
	}
}

func (t *CmdObjectLogs) local(selStr string) error {
	sel := objectselector.New(
		selStr,
		objectselector.WithLocal(true),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	matches := parseFilters(&t.Filter)
	last := len(paths) - 1
	for i, path := range paths {
		matches = append(matches, "OBJ_PATH="+path.String())
		if i > 0 && i < last {
			matches = append(matches, "+")
		}
	}
	stream := streamlog.NewStream()
	streamConfig := streamlog.StreamConfig{
		Follow:  t.Follow,
		Lines:   t.Lines,
		Matches: matches,
	}
	if err := stream.Start(streamConfig); err != nil {
		return err
	}
	defer stream.Stop()
	for {
		select {
		case err := <-stream.Errors():
			if err == nil {
				// The sender has stopped sending
				return nil
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		case ev := <-stream.Events():
			ev.Render(t.Output)
		}
	}
}

func (t *CmdObjectLogs) asCommonCmd() *commoncmd.CmdObjectLogs {
	return &commoncmd.CmdObjectLogs{
		OptsGlobal: commoncmd.OptsGlobal{
			Color:          t.Color,
			Output:         t.Output,
			ObjectSelector: t.ObjectSelector,
		},
		OptsLogs: commoncmd.OptsLogs{
			Follow: t.Follow,
			Lines:  t.Lines,
			Filter: t.Filter,
		},
		NodeSelector: t.NodeSelector,
	}
}
