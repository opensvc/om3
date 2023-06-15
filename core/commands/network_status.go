package commands

import (
	"context"
	"fmt"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
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
	data := network.NewStatusList()
	params := api.GetNetworksParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetNetworks(context.Background(), &params)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var problem api.Problem
		_ = json.NewDecoder(resp.Body).Decode(&problem)
		return data, fmt.Errorf("%s", problem)
	}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return data, fmt.Errorf("Unmarshal GET /networks: %w", err)
	}
	return data, nil
}
