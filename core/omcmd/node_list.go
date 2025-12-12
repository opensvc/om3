package omcmd

import (
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdNodeList struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeList) Run() error {
	var (
		err      error
		selector string
	)
	c, err := client.New()
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		selector = "*"
	} else {
		selector = t.NodeSelector
	}
	nodes, err := nodeselector.New(selector, nodeselector.WithClient(c)).Expand()
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
