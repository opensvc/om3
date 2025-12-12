package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdNetworkList struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdNetworkList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetNetworksParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetNetworksWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	var pb api.Problem
	switch resp.StatusCode() {
	case 200:
		output.Renderer{
			DefaultOutput: "tab=NAME:name,TYPE:type,NETWORK:network,SIZE:size,USED:used,FREE:free",
			Output:        t.Output,
			Color:         t.Color,
			Data:          resp.JSON200,
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
