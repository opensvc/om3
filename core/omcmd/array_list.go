package omcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdArrayList struct {
		OptsGlobal
		Local bool
	}
)

func (t *CmdArrayList) Run() error {
	var (
		data []string
		err  error
	)
	if t.Local {
		data, err = t.extractLocal()
	} else {
		data, err = t.extractDaemon()
	}
	output.Renderer{
		Output: t.Output,
		Color:  t.Color,
		Data:   data,
		HumanRenderer: func() string {
			s := ""
			for _, e := range data {
				s += e + "\n"
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}.Print()
	return err
}

func (t *CmdArrayList) extractLocal() ([]string, error) {
	n, err := object.NewNode()
	if err != nil {
		return []string{}, err
	}
	return n.ListArrays(), nil
}

func (t *CmdArrayList) extractDaemon() ([]string, error) {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(); err != nil {
		return []string{}, err
	}
	return []string{}, fmt.Errorf("todo %v", c)
}
