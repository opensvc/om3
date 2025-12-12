package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdArrayList struct {
		OptsGlobal
	}
)

func (t *CmdArrayList) Run() error {
	var (
		data []string
		err  error
	)
	data, err = t.extractDaemon()
	if err != nil {
		return err
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
	return nil
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
