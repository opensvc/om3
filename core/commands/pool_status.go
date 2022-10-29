package commands

import (
	"encoding/json"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	CmdPoolStatus struct {
		OptsGlobal
		Verbose bool
		Name    string
	}
)

func (t *CmdPoolStatus) Run() error {
	var (
		err  error
		data pool.StatusList
	)
	if !t.Local || clientcontext.IsSet() {
		data, err = t.extractDaemon()
	} else {
		data, err = t.extractLocal()
	}
	if err != nil {
		return err
	}
	output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     data,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return data.Render(t.Verbose)
		},
	}.Print()
	return nil
}

func (t *CmdPoolStatus) extractLocal() (pool.StatusList, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}

	if t.Name == "" {
		return n.ShowPools(), nil
	} else {
		return n.ShowPoolsByName(t.Name), nil
	}
}

func (t *CmdPoolStatus) extractDaemon() (pool.StatusList, error) {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return nil, err
	}
	l := pool.NewStatusList()
	data := make(map[string]pool.Status)
	req := c.NewGetPools()
	req.SetName(t.Name)
	b, err := req.Do()
	if err != nil {
		return l, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return l, errors.Wrapf(err, "unmarshal GET /pools")
	}
	for name, d := range data {
		d.Name = name
		l = append(l, d)
	}
	return l, nil
}
