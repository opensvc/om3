package commands

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		OptsLogs
		NodeSelector string
	}
)

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
			fmt.Fprintln(os.Stderr, err)
			if err == nil {
				// The sender has stopped sending
				return nil
			}
		case ev := <-stream.Events():
			ev.Render(t.Output)
		}
	}
	return nil
}

func (t *CmdNodeLogs) Run() error {
	render.SetColor(t.Color)
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	return t.local()
}
