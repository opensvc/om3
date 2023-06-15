package commands

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdNetworkLs struct {
		OptsGlobal
	}
)

func (t *CmdNetworkLs) Run() error {
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
		Format: t.Format,
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

func (t *CmdNetworkLs) extractLocal() ([]string, error) {
	n, err := object.NewNode()
	if err != nil {
		return []string{}, err
	}
	return network.List(n), nil
}

func (t *CmdNetworkLs) extractDaemon() ([]string, error) {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return []string{}, err
	}
	return []string{}, fmt.Errorf("todo %v", c)
}
