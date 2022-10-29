package commands

import (
	"encoding/json"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	CmdNetworkStatus struct {
		OptsGlobal
		Verbose bool
		Name    string
	}
)

func (t *CmdNetworkStatus) Run() error {
	var (
		err  error
		data network.StatusList
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

func (t *CmdNetworkStatus) extractLocal() (network.StatusList, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	return network.ShowNetworksByName(n, t.Name), nil
}

func (t *CmdNetworkStatus) extractDaemon() (network.StatusList, error) {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return nil, err
	}
	l := network.NewStatusList()
	data := make(map[string]network.Status)
	req := c.NewGetNetworks()
	req.SetName(t.Name)
	b, err := req.Do()
	if err != nil {
		return l, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return l, errors.Wrapf(err, "unmarshal GET /networks")
	}
	for name, d := range data {
		if t.Name != "" && name != t.Name {
			// TODO: api handler should honor the name filter set in request
			continue
		}
		d.Name = name
		l = append(l, d)
	}
	return l, nil
}
