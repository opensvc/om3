package oxcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeCapabilitiesList struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeCapabilitiesList) extract(c *client.T, nodename string) (api.CapabilityList, error) {
	resp, err := c.GetNodeCapabilitiesWithResponse(context.Background(), nodename)
	if err != nil {
		return api.CapabilityList{}, err
	}
	switch resp.StatusCode() {
	case 200:
		return *resp.JSON200, nil
	case 401:
		return api.CapabilityList{}, fmt.Errorf("%s: %s", nodename, *resp.JSON401)
	case 403:
		return api.CapabilityList{}, fmt.Errorf("%s: %s", nodename, *resp.JSON403)
	default:
		return api.CapabilityList{}, fmt.Errorf("%s: unexpected statuscode: %s", nodename, resp.Status())
	}
}

func (t *CmdNodeCapabilitiesList) Run() error {
	var errs error
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	var data api.CapabilityList
	for i, nodename := range nodenames {
		if d, err := t.extract(c, nodename); err != nil {
			errs = errors.Join(err)
		} else if i == 0 {
			data = d
		} else {
			data.Items = append(data.Items, d.Items...)
		}

	}
	output.Renderer{
		DefaultOutput: "tab=NODE:meta.node,NAME:data.name",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}
