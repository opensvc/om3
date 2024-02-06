package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeSystemDisk struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeSystemDisk) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}

	sel := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c))
	nodenames, err := sel.Expand()
	if err != nil {
		return err
	}

	l := make(api.DiskItems, 0)
	for _, nodename := range nodenames {
		response, err := c.GetNodeSystemDiskWithResponse(context.Background(), nodename)
		if err != nil {
			return err
		}
		switch {
		case response.JSON200 != nil:
			l = append(l, response.JSON200.Items...)
		case response.JSON400 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON400)
		case response.JSON401 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON401)
		case response.JSON403 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON403)
		case response.JSON500 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
		}
	}

	defaultOutput := "tab=NODE:meta.node,ID:data.ID,DEVPATH:data.devpath,SIZE:data.size,VENDOR:data.vendor,MODEL:data.model,TYPE:data.type,REGION_ID:data.regions[0].ID,OBJECT:data.regions[0].object,GROUP:data.regions[0].group,REGION_DEVPATH:data.regions[0].devpath,REGION_SIZE:data.regions[0].size"
	output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.DiskList{Items: l, Kind: "DiskList"},
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}
