package commands

import (
	"github.com/pkg/errors"

	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/rawconfig"
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
	l := pool.NewStatusList()
	/*
		c, err := client.New(client.WithURL(t.Server))
		if err != nil {
			return nil, err
		}
		params := api.GetPools{
			Name: t.Name,
		}
		resp, err := c.GetPools(context.Background(), &params)
		if err != nil {
			return l, err
		}
		defer resp.Body.Close()
		data := make(map[string]pool.Status)
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			return l, errors.Wrapf(err, "unmarshal GET /pools")
		}
		for name, d := range data {
			d.Name = name
			l = append(l, d)
		}
		return l, nil
	*/
	return l, errors.Errorf("TODO")
}
