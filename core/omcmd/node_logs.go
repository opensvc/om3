package omcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		commoncmd.OptsLogs
		Local        bool
		NodeSelector string
	}
)

func (t *CmdNodeLogs) Run() error {
	render.SetColor(t.Color)
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	if t.Local {
		return t.local()
	} else {
		return t.asCommonCmd().Remote()
	}
}

func parseFilters(l *[]string) []string {
	m := make([]string, 0)
	if l == nil {
		return m
	}
	return *l
}

func (t *CmdNodeLogs) local() error {
	matches := parseFilters(&t.Filter)
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

func (t *CmdNodeLogs) asCommonCmd() *commoncmd.CmdNodeLogs {
	return &commoncmd.CmdNodeLogs{
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
