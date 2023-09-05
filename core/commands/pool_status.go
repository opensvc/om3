package commands

import (
	"context"
	"fmt"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
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
		Output:   t.Output,
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
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return nil, err
	}
	params := api.GetPoolsParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetPoolsWithResponse(context.Background(), &params)
	if err != nil {
		return l, err
	}
	switch resp.StatusCode() {
	case 200:
		if err := json.Unmarshal(resp.Body, &l); err != nil {
			return l, fmt.Errorf("unmarshal GET /pools: %w", err)
		}
		return l, nil
	case 401:
		return l, fmt.Errorf("%s", resp.JSON401)
	case 403:
		return l, fmt.Errorf("%s", resp.JSON403)
	case 500:
		return l, fmt.Errorf("%s", resp.JSON500)
	default:
		return l, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
}
