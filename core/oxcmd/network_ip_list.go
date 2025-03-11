package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNetworkIPList struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdNetworkIPList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetNetworkIPParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetNetworkIPWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	var pb api.Problem
	switch resp.StatusCode() {
	case 200:
		output.Renderer{
			DefaultOutput: "tab=OBJECT:path,NODE:node,RID:rid,IP:ip,NET_NAME:network.name,NET_TYPE:network.type",
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
