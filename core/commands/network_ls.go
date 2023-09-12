package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNetworkLs struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdNetworkLs) Run() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	params := api.GetNetworkParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetNetworkWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	var pb api.Problem
	switch resp.StatusCode() {
	case 200:
		data := *resp.JSON200
		output.Renderer{
			DefaultOutput: "tab=NAME:name,TYPE:type,NETWORK:network,SIZE:usage.size,USED:usage.used,FREE:usage.free,USE_PCT:usage.pct",
			Output:        t.Output,
			Color:         t.Color,
			Data:          data,
			Colorize:      rawconfig.Colorize,
		}.Print()
		return nil
	case 401:
		pb = *resp.JSON401
	case 403:
		pb = *resp.JSON403
	case 500:
		pb = *resp.JSON500
	}
	return fmt.Errorf("%s", pb)
}
