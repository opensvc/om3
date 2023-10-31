package commands

import (
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdNodeLs struct {
		OptsGlobal
	}
)

func (t *CmdNodeLs) Run() error {
	var (
		err      error
		selector string
	)
	if t.NodeSelector == "" {
		selector = "*"
	} else {
		selector = t.NodeSelector
	}
	nodes, err := nodeselector.Expand(selector)
	if err != nil {
		return err
	}
	output.Renderer{
		Output: t.Output,
		Color:  t.Color,
		Data:   nodes,
		HumanRenderer: func() string {
			s := ""
			for _, e := range nodes {
				s += e + "\n"
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}.Print()
	return nil
}
