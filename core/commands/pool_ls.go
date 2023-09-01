package commands

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdPoolLs struct {
		OptsGlobal
	}
)

func (t *CmdPoolLs) Run() error {
	var (
		data []string
		err  error
	)
	if t.Local || !clientcontext.IsSet() {
		data, err = t.extractLocal()
	} else {
		data, err = t.extractDaemon()
	}
	output.Renderer{
		Format: t.Output,
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

func (t *CmdPoolLs) extractLocal() ([]string, error) {
	n, err := object.NewNode()
	if err != nil {
		return []string{}, err
	}
	return n.ListPools(), nil
}

func (t *CmdPoolLs) extractDaemon() ([]string, error) {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return []string{}, err
	}
	return []string{}, fmt.Errorf("todo %v", c)
}
