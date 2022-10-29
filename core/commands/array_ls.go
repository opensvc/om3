package commands

import (
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	CmdArrayLs struct {
		OptsGlobal
	}
)

func (t *CmdArrayLs) Run() error {
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

func (t *CmdArrayLs) extractLocal() ([]string, error) {
	n, err := object.NewNode()
	if err != nil {
		return []string{}, err
	}
	return n.ListArrays(), nil
}

func (t *CmdArrayLs) extractDaemon() ([]string, error) {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return []string{}, err
	}
	return []string{}, errors.Errorf("TODO", c)
}
